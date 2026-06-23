# 11. Automatic Failure Recovery Control Plane

This chapter explains the automatic failure recovery feature at the level a developer needs before
changing it. The short version is: MnemoKV still routes client commands through the existing
cluster manager and fixed-slot metadata, while a separate embedded controller observes node health,
commits recovery decisions through Raft, and invokes the same promotion, repair, and sync APIs that
manual failover already uses.

Automatic recovery is opt-in. Manual mode remains the default and still uses unsigned operator
calls. Automatic mode is selected only at startup with `cluster.failoverMode: automatic`.

## Design Boundaries

The controller is intentionally decoupled from the data path:

- RESP and HTTP commands still enter through `internal/server`, `internal/api`, and
  `internal/cluster.Coordinator`.
- Slot ownership still lives in `internal/cluster.Metadata`.
- Data copying still uses the existing full-slot synchronization path in `internal/cluster/repair.go`.
- Raft is control-plane-only. It commits observations, plans, and step-completion markers; it does
  not replicate user key/value data.
- Automatic mode authenticates topology-changing node API calls with controller HMAC headers and a
  monotonic control index.

That separation is the main safety property. The controller may be stopped or leaderless, and
unaffected slots can continue serving from their last committed cluster metadata. Without a Raft
majority, ownership changes freeze instead of being guessed locally.

## Main Modules

`cmd/node/main.go`

Creates the controller only when the cluster is enabled and `failoverMode` is `automatic`. Manual
startup does not construct it. The controller starts after the cluster manager so it can observe the
local data node and call the local API.

`internal/config`

Defines and validates the static automatic-mode contract:

- five consistently configured peers for the demo topology;
- unique data/API/control addresses;
- a shared `controller.bootstrapNodeId`;
- a per-node `controller.raftDir`;
- positive observation, failure, and rebalance settings;
- a shared non-empty `controlPlane.requestSigningSecret`.

The project does not expose a runtime mode-switch endpoint. Changing mode is an operator procedure:
make the cluster healthy, stop all nodes, change every YAML consistently, and restart the full
cluster.

`internal/controller/raftnode.go`

Wraps Hashicorp Raft. Production nodes use a TCP transport on each configured control address and a
BoltDB log/stable store under `controller.raftDir`. Tests inject in-memory transports and stores.
Only the configured bootstrap node bootstraps a fresh Raft cluster, and only when there is no
existing Raft state. Existing state wins over YAML bootstrap settings so a restarted voter keeps its
identity.

`internal/controller/fsm.go`

Implements the deterministic Raft FSM. It applies commands such as:

- committed cluster observations;
- proposed recovery or rebalance plans;
- completed step markers;
- plan completion or supersession;
- returning-node admission.

The FSM performs no network I/O. It only mutates committed control-plane state. This keeps replay,
snapshot, and restore deterministic.

`internal/controller/observer.go`

Polls each peer's `/health` and `/cluster/state` with short timeouts. It tracks suspected and
confirmed failures, chooses the highest compatible metadata version seen from reachable peers, and
builds the committed `ClusterView` used by the planner. The leader proposes a new observation only
when the material view changes.

`internal/controller/topology.go`

Classifies the committed view into configured, voter, eligible, unavailable, and returning nodes.
It also classifies every slot:

- `Unaffected`: leader and ready replica are reachable.
- `Leaderless`: leader is unavailable, replica is reachable.
- `ReplicaLost`: leader is reachable, replica is unavailable.
- `NoSurvivingCopy`: neither authoritative copy is reachable.

This classification is recomputed on every committed view change. Failed nodes are excluded from
placement but remain configured members.

`internal/controller/planner.go`

Contains pure planning algorithms. It emits no network calls and is heavily unit-tested.

For recovery:

- `Leaderless` slots get `Promote`, then `AssignReplica`, then `Sync`.
- `ReplicaLost` slots get `AssignReplica`, then `Sync`.
- `NoSurvivingCopy` slots get `MarkUnavailable`; the controller never creates an empty leader.
- Replacement targets are the least-loaded eligible nodes, with deterministic node-ID tie breaks.

For rebalancing:

- the planner counts leaders and replicas over eligible nodes;
- it compares max/min skew against `rebalanceSkewThreshold`;
- it generates capped deterministic moves;
- leadership moves are expressed as safe hand-offs:
  `AssignReplica(target) -> Sync(target) -> Promote(target) -> AssignReplica(oldLeader) -> Sync`.

The same executor handles recovery and rebalancing plans.

`internal/controller/executor.go`

Runs the active committed plan in order. Each external operation is fenced with:

- `X-MnemoKV-Control-Index`;
- a matching HMAC signature over method, path, body, and index.

The executor is idempotent. Ambiguous API responses do not automatically become successful steps.
Instead, the executor fetches current metadata and commits `StepDone` only if the exact postcondition
already holds. On leadership changes, the new leader resumes from the first not-yet-done step.

`internal/controller/returning_node.go`

Handles the fresh returning-node lifecycle. A restarted automatic node keeps its node ID and Raft
identity, but its old application data and snapshots are not trusted. The node is admitted only
after it is empty, has current metadata, and is safe to receive slots through normal synchronization.
Returning stale data is never used to resolve `potential_data_loss`.

`internal/controlplane`

Defines shared controller status contracts and request-signing/fencing helpers. Data-node API
handlers use these helpers to reject unsigned, forged, stale, or conflicting topology requests in
automatic mode.

`internal/api`

Exposes the read-only observability surface:

- `/cluster/state` includes recovery status, affected slots, one-copy windows, and
  potential-data-loss warnings;
- `/controller/state` exposes Raft role/leader/term, the committed view, active plan progress,
  unavailable slots, and the last completed rebalance.

Manual topology endpoints remain available only in manual mode. In automatic mode, topology
mutation requires valid controller authentication.

`frontend/src/pages/use/ClusterPage.tsx`

Displays the automatic recovery state on the cluster page: degraded windows, active-plan progress,
one-copy slots, no-copy slots, failed nodes, returning-node policy, and warnings about the
one-failure-at-a-time guarantee.

## Recovery Algorithm

The recovery loop is:

1. Observe peer health and cluster state.
2. Commit the material view through Raft.
3. Classify topology and slots from the committed view.
4. If confirmed failures exist and no compatible active plan exists, commit a recovery plan.
5. Execute committed steps serially with authenticated, monotonic fencing.
6. After each external mutation, verify metadata convergence on reachable eligible nodes.
7. Commit `StepDone` only after the step postcondition is true.
8. Commit `PlanComplete` when every step is done.
9. If the eligible topology is stable but ownership is skewed, commit and execute a rebalance plan.
10. Report `healthy` only when every non-unavailable slot has a leader and ready distinct replica,
    failed nodes own zero slots, and placement is within the configured skew threshold.

The controller never treats a transient local observation as authority. Planning follows committed
views, and data-node mutations follow committed plans.

## Status States

The public states are:

- `healthy`: all slots have a leader and ready replica on distinct eligible nodes.
- `failure_suspected`: observation has misses but the failure timeout has not confirmed them.
- `degraded`: at least one slot has only one reachable authoritative copy.
- `promoting`: a plan is restoring leadership for leaderless slots.
- `repairing`: a plan is assigning and synchronizing replacement replicas.
- `rebalancing`: recovery is complete and placement is being made fair again.
- `unavailable`: at least one slot has no currently reachable leader.
- `potential_data_loss`: at least one slot has no reachable authoritative copy.

These names are shared by API responses, SSE snapshots, metrics, logs, and the frontend.

## Guarantees And Non-Guarantees

Supported guarantee: one data-node failure at a time, with full repair before the next destructive
failure. During promotion and repair there is a real degraded window. Reads may continue from a
surviving leader, but writes to a slot without a ready replica are rejected by the existing cluster
write-safety rule.

Unsupported v1 case: a second destructive failure before repair completes may leave some slots with
no reachable authoritative copy. The controller reports those slots as `potential_data_loss` and
does not assign empty leaders. A returning node's stale memory or snapshots are not used to recover
those slots.

## Tests And Demo Entry Points

The fastest code-reading path is:

- `internal/controller/planner_test.go` for slot classification, recovery planning, and rebalance
  planning;
- `internal/controller/fsm_test.go` for deterministic Raft FSM behavior;
- `internal/controller/executor_test.go` for idempotent execution and postcondition checks;
- `internal/controller/harness_test.go` for five-node in-process scenarios;
- `internal/api/controller_status_test.go` for API observability contracts;
- `frontend/e2e/app.spec.ts` for the cluster-page recovery display.

For process-level verification, use:

```powershell
.\scripts\demo-automatic-recovery.ps1
.\scripts\demo-automatic-recovery.ps1 -ReturnNode
```

The first command demonstrates automatic promotion, repair, and four-node rebalancing after a
leader-node failure. The second also restarts the failed node and demonstrates fresh admission plus
4-to-5 scale-back.
