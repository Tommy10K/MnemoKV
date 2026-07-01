# Run And Use MnemoKV

Last reviewed against the codebase: July 1, 2026.

This file is a practical command cookbook. It explains how to start each project mode, how to send
commands, how to inspect the node, and how to repair a manual cluster. Commands are written for
PowerShell from the repository root unless noted otherwise.

## 1. Requirements And Mental Model

You need:

- Go 1.22 or newer.
- Node.js and npm for the frontend.
- PowerShell 7 for the bundled Windows demo scripts.
- `redis-cli` is optional but useful because MnemoKV speaks RESP2.

MnemoKV has two public interfaces:

- RESP TCP, used by `redis-cli`, workloads, and normal database clients.
- HTTP API, used by the frontend, demos, and `cmd/adminctl`.

The default standalone ports are:

| Interface | Default address |
| --- | --- |
| RESP | `127.0.0.1:6380` |
| HTTP API | `http://127.0.0.1:7380` |
| Frontend dev server | usually `http://localhost:5173` |

Cluster configs use predictable port ranges:

| Node | Manual RESP | Manual API | Automatic RESP | Automatic API | Automatic controller |
| --- | ---: | ---: | ---: | ---: | ---: |
| `node-1` | `6381` | `7381` | `6381` | `7381` | `7481` |
| `node-2` | `6382` | `7382` | `6382` | `7382` | `7482` |
| `node-3` | `6383` | `7383` | `6383` | `7383` | `7483` |
| `node-4` | n/a | n/a | `6384` | `7384` | `7484` |
| `node-5` | n/a | n/a | `6385` | `7385` | `7485` |

If something will not start, first check whether an old process is still using one of those ports.

## 2. Building Or Running Without Building

There are two normal ways to run backend tools.

### Option A: `go run`

This compiles and runs in one step. It is simple during development:

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

Use this when you only need one process or you do not care about startup speed.

### Option B: build binaries first

This is better for repeated demos or multi-process clusters:

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
go build -o bin/mnemokv-node.exe ./cmd/node
go build -o bin/mnemokv-workload.exe ./cmd/workload
go build -o bin/mnemokv-adminctl.exe ./cmd/adminctl
```

Then run the built binaries:

```powershell
.\bin\mnemokv-node.exe -config configs/standalone.yaml
.\bin\mnemokv-adminctl.exe -port 7380 health
.\bin\mnemokv-workload.exe -addr 127.0.0.1:6380 -profile mixed -duration 10s
```

The source-equivalent commands are:

```powershell
go run ./cmd/adminctl -port 7380 health
go run ./cmd/workload -addr 127.0.0.1:6380 -profile mixed -duration 10s
```

## 3. Ways To Send Database Commands

Use whichever surface is most convenient.

### Option A: `redis-cli`

```powershell
redis-cli -p 6380 PING
redis-cli -p 6380 SET greeting hello
redis-cli -p 6380 GET greeting
redis-cli -p 6380 INCR visits
redis-cli -p 6380 RPUSH queue first second
redis-cli -p 6380 ZADD scores 10 alice 20 bob
redis-cli -p 6380 ZRANGE scores 0 -1 WITHSCORES
```

For a cluster node, change the port:

```powershell
redis-cli -p 6381 SET cluster:key value
redis-cli -p 6382 GET cluster:key
redis-cli -p 6383 GET cluster:key
```

In cluster mode, any node can be used as a gateway. It will route the command to the slot leader.

### Option B: HTTP `/commands`

The HTTP API accepts JSON with an `args` array:

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7380/commands `
  -ContentType application/json `
  -Body '{"args":["SET","greeting","hello"]}'

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7380/commands `
  -ContentType application/json `
  -Body '{"args":["GET","greeting"]}'
```

For a cluster gateway, change the API port:

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7382/commands `
  -ContentType application/json `
  -Body '{"args":["SET","cluster:key","value"]}'
```

### Option C: frontend console

Start the frontend, open `/use/console`, and type commands such as:

```text
SET greeting hello
GET greeting
RPUSH queue first second
ZRANGE scores 0 -1 WITHSCORES
```

The frontend sends the same `/commands` request shown above.

## 4. Inspecting A Running Node

### With `adminctl`

`adminctl` talks to the HTTP API and pretty-prints JSON:

```powershell
go run ./cmd/adminctl -port 7380 health
go run ./cmd/adminctl -port 7380 engine
go run ./cmd/adminctl -port 7380 metrics
go run ./cmd/adminctl -port 7380 snapshot
```

For a cluster node:

```powershell
go run ./cmd/adminctl -port 7381 cluster
go run ./cmd/adminctl -port 7382 cluster
go run ./cmd/adminctl -port 7383 cluster
```

If you built binaries:

```powershell
.\bin\mnemokv-adminctl.exe -port 7381 cluster
```

### With raw HTTP

```powershell
Invoke-RestMethod http://127.0.0.1:7380/health
Invoke-RestMethod http://127.0.0.1:7380/engine/state
Invoke-RestMethod http://127.0.0.1:7380/metrics/summary
Invoke-RestMethod http://127.0.0.1:7381/cluster/state
```

For readable nested JSON:

```powershell
Invoke-RestMethod http://127.0.0.1:7381/cluster/state | ConvertTo-Json -Depth 8
```

Automatic-mode controller status:

```powershell
Invoke-RestMethod http://127.0.0.1:7382/controller/state | ConvertTo-Json -Depth 8
```

## 5. Frontend

Start a backend first, then open a second terminal:

```powershell
cd frontend
npm.cmd ci
npm.cmd run dev
```

If dependencies are already installed, this is also fine:

```powershell
cd frontend
npm.cmd install
npm.cmd run dev
```

Useful frontend routes:

- `/learn` - educational chapters.
- `/use` - configuration guidance.
- `/use/dashboard` - health, memory, metrics, and SSE status.
- `/use/console` - browser command console.
- `/use/workloads` - workload command builder.
- `/use/cluster` - cluster topology and recovery status.
- `/use/eviction` - runtime eviction policy lab.
- `/use/benchmarks` - benchmark import and charts.

The frontend defaults to `http://127.0.0.1:7380`. Change the API target in the Use section when you
want to connect to a cluster node, for example `http://127.0.0.1:7381`.

To set the initial API URL before startup:

```powershell
$env:VITE_API_BASE_URL = "http://127.0.0.1:7381"
npm.cmd run dev
```

Frontend checks:

```powershell
cd frontend
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
npm.cmd run test:e2e:headed
```

`test:e2e:headed` opens the browser visibly.

## 6. Config: `configs/standalone.yaml`

Use this for the normal single-node database.

Start it:

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

Or with a built binary:

```powershell
.\bin\mnemokv-node.exe -config configs/standalone.yaml
```

Endpoints:

- RESP: `127.0.0.1:6380`
- HTTP: `http://127.0.0.1:7380`

Try commands:

```powershell
redis-cli -p 6380 PING
redis-cli -p 6380 SET greeting hello
redis-cli -p 6380 GET greeting
```

Same thing through HTTP:

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7380/commands `
  -ContentType application/json `
  -Body '{"args":["PING"]}'
```

Load deterministic presentation data:

```powershell
.\scripts\load-demo-dataset.ps1
```

With overrides:

```powershell
.\scripts\load-demo-dataset.ps1 `
  -ApiBaseUrl http://127.0.0.1:7380 `
  -Dataset .\examples\demo-dataset.json
```

Run the standalone demo:

```powershell
.\scripts\demo-standalone.ps1
```

The standalone demo assumes the standalone node is already running. It verifies strings, TTL, lists,
sorted sets, malformed RESP handling, a short workload, and metrics.

## 7. Config: `configs/standalone-low-memory.yaml`

Use this to demonstrate hard accounted memory limits and eviction.

Stop any existing standalone node first because this config uses the same ports `6380` and `7380`.

Start the low-memory node:

```powershell
go run ./cmd/node -config configs/standalone-low-memory.yaml
```

Then run:

```powershell
.\scripts\demo-low-memory.ps1
```

What it shows:

- initial policy is `noeviction`;
- a write that would exceed the 512-byte accounted limit is rejected;
- existing keys are preserved;
- switching to `lru` lets the new write in after evicting a victim.

You can switch policy manually too:

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7380/engine/eviction-policy `
  -ContentType application/json `
  -Body '{"policy":"lru"}'
```

Valid policy names:

- `noeviction`
- `fifo`
- `lru`
- `lfu`
- `random`

## 8. Configs: Persistence JSON And Binary

Use these to demonstrate snapshot persistence:

- `configs/standalone-persistence-json.yaml`
- `configs/standalone-persistence-binary.yaml`

Both use the same default standalone ports, so run only one at a time.

Manual JSON run:

```powershell
go run ./cmd/node -config configs/standalone-persistence-json.yaml
```

Manual binary run:

```powershell
go run ./cmd/node -config configs/standalone-persistence-binary.yaml
```

Create a snapshot:

```powershell
go run ./cmd/adminctl -port 7380 snapshot
```

Equivalent raw HTTP:

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:7380/admin/snapshot
```

Automated demo for both formats:

```powershell
.\scripts\demo-persistence.ps1
```

Only JSON:

```powershell
.\scripts\demo-persistence.ps1 -Format json
```

Only binary:

```powershell
.\scripts\demo-persistence.ps1 -Format binary
```

Important limitation: snapshots are not a write-ahead log. Writes after the latest snapshot can be
lost on crash.

## 9. Configs: Manual Cluster

Manual cluster configs:

- `configs/cluster-node-1.yaml`
- `configs/cluster-node-2.yaml`
- `configs/cluster-node-3.yaml`

This is a three-node cluster using:

- fixed 1,024 slots;
- any-node proxy routing;
- one leader and one synchronous replica per slot;
- manual promotion, replacement assignment, and full-slot sync.

### Start a manual cluster by hand

Open three terminals from the repository root.

Terminal 1:

```powershell
go run ./cmd/node -config configs/cluster-node-1.yaml
```

Terminal 2:

```powershell
go run ./cmd/node -config configs/cluster-node-2.yaml
```

Terminal 3:

```powershell
go run ./cmd/node -config configs/cluster-node-3.yaml
```

Check health from a fourth terminal:

```powershell
Invoke-RestMethod http://127.0.0.1:7381/health
Invoke-RestMethod http://127.0.0.1:7382/health
Invoke-RestMethod http://127.0.0.1:7383/health
```

Check the slot table:

```powershell
go run ./cmd/adminctl -port 7381 cluster
```

Or:

```powershell
Invoke-RestMethod http://127.0.0.1:7381/cluster/state | ConvertTo-Json -Depth 8
```

Send commands through any gateway:

```powershell
redis-cli -p 6381 SET manual:cluster:key value
redis-cli -p 6382 GET manual:cluster:key
redis-cli -p 6383 GET manual:cluster:key
```

HTTP equivalent:

```powershell
Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7383/commands `
  -ContentType application/json `
  -Body '{"args":["SET","manual:cluster:key","value"]}'
```

### Start a manual cluster with the demo script

The script builds a node binary, starts three isolated temporary nodes, verifies routing and strict
replication, then cleans up:

```powershell
.\scripts\demo-cluster.ps1
```

Keep the nodes running for inspection:

```powershell
.\scripts\demo-cluster.ps1 -KeepRunning
```

Linux/macOS helper:

```bash
./scripts/run-cluster.sh
```

Stop one Linux/macOS helper node:

```bash
./scripts/kill-node.sh 2
```

### Manual cluster repair: what the three repair commands mean

Manual mode has no automatic election. If a node fails, an operator repairs affected slots. The
three topology commands are:

1. `cluster-promote <slot>`: promote the slot's current replica to leader.
2. `cluster-assign-replica <slot> <node>`: choose a new replacement replica.
3. `cluster-sync <slot> [node]`: copy the full slot from the current leader to the replacement and
   mark the replica ready. This request must be sent to the current leader's API port for that
   slot.

Writes to a slot are rejected while that slot has no ready replica. Reads can continue only when
the request can reach the current slot leader.

There is no bulk repair command. For a real node failure, repeat the appropriate steps for every
affected slot shown by `/cluster/state`.

### Manual repair case A: leader failed, replica is alive

Example situation:

- `node-1` died.
- A slot has `leaderId = node-1`.
- That same slot has `replicaId = node-2`.
- `node-3` is alive and can become the new replica.

First inspect affected slots from a live node:

```powershell
$state = Invoke-RestMethod http://127.0.0.1:7382/cluster/state
$state.slots | Where-Object { $_.leaderId -eq "node-1" } | Select-Object -First 10
```

Pick one affected slot. In the default slot layout, slot `42` is usually in `node-1`'s leader range,
but do not guess during a real run; inspect `/cluster/state`.

Repair one slot with `adminctl`:

```powershell
$slot = 42
go run ./cmd/adminctl -port 7382 cluster-promote $slot
go run ./cmd/adminctl -port 7382 cluster-assign-replica $slot node-3
go run ./cmd/adminctl -port 7382 cluster-sync $slot node-3
```

The same repair through raw HTTP:

```powershell
$slot = 42

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7382/cluster/promote `
  -ContentType application/json `
  -Body "{`"slot`":$slot}"

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7382/cluster/replica `
  -ContentType application/json `
  -Body "{`"slot`":$slot,`"nodeId`":`"node-3`"}"

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7382/cluster/sync `
  -ContentType application/json `
  -Body "{`"slot`":$slot,`"nodeId`":`"node-3`"}"
```

Verify:

```powershell
$state = Invoke-RestMethod http://127.0.0.1:7382/cluster/state
$state.slots[$slot]
```

You want:

- `leaderId` to be the promoted former replica;
- `replicaId` to be the replacement node;
- `replicaReady` to be `true`;
- `term` and `metadataVersion` to have advanced.

### Manual repair case B: replica failed, leader is alive

If the leader is alive and only the replica failed, do not promote. Assign a replacement and sync.

Example:

```powershell
$state = Invoke-RestMethod http://127.0.0.1:7381/cluster/state
$state.slots |
  Where-Object { $_.replicaId -eq "node-2" -and $_.leaderId -eq "node-1" } |
  Select-Object -First 10
```

For one affected slot, repair it:

```powershell
$slot = 42
go run ./cmd/adminctl -port 7381 cluster-assign-replica $slot node-3
go run ./cmd/adminctl -port 7381 cluster-sync $slot node-3
```

Use the API port of the current slot leader for `cluster-sync`. In the example above, the selected
slot is filtered to `leaderId = node-1`, so API port `7381` is correct.

### Manual repair case C: both authoritative copies are gone

If both the leader and the replica for a slot are unavailable, manual mode cannot reconstruct that
slot's data from nowhere. Do not assign an empty leader and pretend the data is recovered. Restore
from a snapshot or restart a node that still has an authoritative copy, then inspect metadata before
repairing.

## 10. Configs: Automatic Cluster

Automatic configs:

- `configs/cluster-node-1-auto.yaml`
- `configs/cluster-node-2-auto.yaml`
- `configs/cluster-node-3-auto.yaml`
- `configs/cluster-node-4-auto.yaml`
- `configs/cluster-node-5-auto.yaml`

Automatic mode starts the embedded Raft controller. It observes failures, commits a recovery plan,
promotes surviving replicas, assigns and syncs replacement replicas, and rebalances ownership. It
does not put user values into Raft.

The supported data guarantee is one node failure at a time with repair in between. During the
degraded window, writes to slots without a ready replica are rejected. A second destructive failure
before repair may cause `potential_data_loss` for affected slots.

### Recommended automatic demo

Use the script first. It is safer than manually coordinating five terminals:

```powershell
.\scripts\demo-automatic-recovery.ps1
```

Keep nodes running after the demo:

```powershell
.\scripts\demo-automatic-recovery.ps1 -KeepRunning
```

Demonstrate a failed node returning fresh and being rebalanced back into the cluster:

```powershell
.\scripts\demo-automatic-recovery.ps1 -ReturnNode
```

Return-node demo and keep the cluster up:

```powershell
.\scripts\demo-automatic-recovery.ps1 -ReturnNode -KeepRunning
```

Watch status from another terminal:

```powershell
Invoke-RestMethod http://127.0.0.1:7382/cluster/state | ConvertTo-Json -Depth 8
Invoke-RestMethod http://127.0.0.1:7382/controller/state | ConvertTo-Json -Depth 8
```

### Start an automatic cluster by hand

For a clean first run, remove old demo data. Only do this for disposable local demo data:

```powershell
Remove-Item -Recurse -Force .\data\auto -ErrorAction SilentlyContinue
```

Open five terminals from the repository root.

Terminal 1:

```powershell
go run ./cmd/node -config configs/cluster-node-1-auto.yaml
```

Terminal 2:

```powershell
go run ./cmd/node -config configs/cluster-node-2-auto.yaml
```

Terminal 3:

```powershell
go run ./cmd/node -config configs/cluster-node-3-auto.yaml
```

Terminal 4:

```powershell
go run ./cmd/node -config configs/cluster-node-4-auto.yaml
```

Terminal 5:

```powershell
go run ./cmd/node -config configs/cluster-node-5-auto.yaml
```

Check the controller:

```powershell
Invoke-RestMethod http://127.0.0.1:7381/controller/state | ConvertTo-Json -Depth 8
Invoke-RestMethod http://127.0.0.1:7382/controller/state | ConvertTo-Json -Depth 8
```

Look for one node where `isLeader` is `true`. The cluster can change ownership only while at least
three Raft voters are available.

Simulate a failure by stopping one node terminal with `Ctrl+C`. Then watch a live node:

```powershell
while ($true) {
  $state = Invoke-RestMethod http://127.0.0.1:7382/cluster/state
  $state.recovery
  Start-Sleep -Seconds 1
}
```

Expected states include:

- `failure_suspected`
- `unavailable`
- `promoting`
- `repairing`
- `rebalancing`
- `healthy`

Manual topology commands are intentionally rejected in automatic mode unless they carry valid
controller authentication. In automatic mode, use the controller and status endpoints, not
`adminctl cluster-promote`.

## 11. Workloads

The workload tool speaks RESP to a running node.

Default:

```powershell
go run ./cmd/workload
```

Explicit standalone run:

```powershell
go run ./cmd/workload `
  -addr 127.0.0.1:6380 `
  -profile mixed `
  -concurrency 8 `
  -duration 10s `
  -keyspan 1000 `
  -seed 42
```

Profiles:

- `strings`
- `lists`
- `zset`
- `mixed`

Against a cluster, point at any node's RESP port:

```powershell
go run ./cmd/workload -addr 127.0.0.1:6381 -profile mixed -duration 15s -seed 42
go run ./cmd/workload -addr 127.0.0.1:6382 -profile strings -concurrency 4 -duration 30s
```

Built binary equivalent:

```powershell
.\bin\mnemokv-workload.exe -addr 127.0.0.1:6380 -profile mixed -duration 10s
```

## 12. Benchmarks

Run Go benchmarks:

```powershell
go test ./internal/engine -bench . -benchmem
```

Save output:

```powershell
go test ./internal/engine -bench . -benchmem | Tee-Object engine-bench.txt
```

Open `/use/benchmarks` and paste/import the output. The page also has a built-in example for a
repeatable presentation without running benchmarks live.

## 13. Test And Validation Commands

Backend:

```powershell
go test ./...
go vet ./...
go test -race ./...
```

Focused concurrent cluster/controller path:

```powershell
go test -race ./internal/controller/... ./internal/cluster/... ./internal/api/...
```

Frontend:

```powershell
cd frontend
npm.cmd ci
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

Cross-mode demos:

```powershell
.\scripts\demo-standalone.ps1
.\scripts\demo-low-memory.ps1
.\scripts\demo-persistence.ps1
.\scripts\demo-cluster.ps1
.\scripts\demo-automatic-recovery.ps1
```

## 14. Common Problems

- If a node fails to bind, another process is probably using the configured RESP/API/controller port.
- If the frontend says offline, check `Invoke-RestMethod http://127.0.0.1:7380/health` and confirm
  the frontend API target points at the right node.
- If `redis-cli` cannot connect, confirm you are using the RESP port, not the HTTP API port.
- If `adminctl` cannot connect, confirm you are using the HTTP API port, not the RESP port.
- If a cluster write fails, inspect the slot in `/cluster/state`; strict replication rejects writes
  when the assigned replica is unavailable or not ready.
- In manual cluster mode, membership health is only a hint. It does not repair ownership.
- In automatic cluster mode, no ownership change can commit without a controller majority.
- PowerShell scripts may require an execution policy that permits local scripts.
- The persistence configs reuse port `6380`/`7380`; stop other standalone nodes before starting them.
- Snapshot persistence is not a write-ahead log. It restores the latest completed snapshot only.
