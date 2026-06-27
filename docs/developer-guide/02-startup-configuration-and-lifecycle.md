# 2. Startup, Configuration, And Lifecycle

## Configuration Is A Startup Contract

Every node starts from one YAML file. [`internal/config/config.go`](../../internal/config/config.go)
defines the complete model: node identity, RESP networking, engine settings, cluster topology,
persistence, and observability.

[`internal/config/loader.go`](../../internal/config/loader.go) loads defaults, decodes YAML with
`KnownFields(true)`, rejects additional YAML documents, applies fallbacks, and calls validation.
This means misspelled fields fail startup instead of being ignored.

[`internal/config/validate.go`](../../internal/config/validate.go) enforces cross-field rules. Read it
beside [ADR 001](../adr/001-system-modes.md).

## Supported Modes

Standalone mode requires cluster flags to be disabled and the peer list to be empty. Cluster mode
requires sharding and replication, `routingMode: proxy`, a cluster ID, a fixed slot count, and two
to five unique peers including the local node.

`cluster.failoverMode` selects the cluster control model at startup:

- `manual` keeps the operator-driven promotion, replica-assignment, and sync APIs.
- `automatic` starts the embedded Raft-backed controller, requires consistently configured automatic
  peers with control addresses and Raft directories, and fences topology mutations behind controller
  signatures.

The `node.mode` string is descriptive. `cluster.enabled` and its validated companion fields choose
the real execution mode.

Use these examples as known-good starting points:

- [`configs/standalone.yaml`](../../configs/standalone.yaml)
- [`configs/standalone-low-memory.yaml`](../../configs/standalone-low-memory.yaml)
- [`configs/standalone-persistence-json.yaml`](../../configs/standalone-persistence-json.yaml)
- [`configs/standalone-persistence-binary.yaml`](../../configs/standalone-persistence-binary.yaml)
- [`configs/cluster-node-1.yaml`](../../configs/cluster-node-1.yaml) and its node-2/node-3 peers
- [`configs/cluster-node-1-auto.yaml`](../../configs/cluster-node-1-auto.yaml) and its node-2
  through node-5 automatic peers

## Composition In `main`

Follow [`cmd/node/main.go`](../../cmd/node/main.go) in this order:

1. Load and validate config.
2. Apply the configured logging level through [`internal/logging`](../../internal/logging/).
3. Create a bounded in-memory metrics sink.
4. Create the engine with the configured stripe count, memory limit, and eviction policy.
5. Create the cluster manager and attach the engine. Disabled cluster managers are inert.
6. Create persistence and provide callbacks to save and restore cluster metadata.
7. Restore the newest valid snapshot before opening listeners when configured.
8. Select `engine.Engine` or `cluster.Coordinator` as the command executor.
9. Construct both RESP and HTTP servers with that executor.
10. Start cluster metadata synchronization, membership probes, optional automatic controller,
    periodic snapshots, and listeners.

This order matters. Restoring metadata before serving cluster traffic prevents a node from briefly
serving with stale slot ownership.

## Go Lifecycle Patterns Used

- `signal.NotifyContext` converts SIGINT/SIGTERM into context cancellation.
- Long-running components accept a `context.Context` and stop when it is cancelled.
- Listener loops run in goroutines and report terminal errors through buffered channels.
- `sync.WaitGroup` waits for connection handlers, membership probes, and snapshot loops.
- Shutdown gets a bounded five-second context so the process cannot wait forever.

The RESP server closes its listener and active sockets. The HTTP server uses
`http.Server.Shutdown`. The cluster manager stops probes and closes cached peer connections. The
persistence manager waits for its periodic loop.

## Startup Commands

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

The default standalone endpoints are RESP `127.0.0.1:6380` and HTTP
`http://127.0.0.1:7380`.

For a production-style local build rather than `go run`:

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
go build -o bin/mnemokv-node.exe ./cmd/node
go build -o bin/mnemokv-workload.exe ./cmd/workload
go build -o bin/mnemokv-adminctl.exe ./cmd/adminctl
```

## What To Change Carefully

- Adding config fields requires model, defaults, validation, example configs, frontend generation,
  tests, and documentation updates.
- Do not silently accept unsupported combinations; explicit startup failure is part of the design.
- A setting used only at startup must not be presented as live-reconfigurable in the frontend.
- Keep the supported modes narrow. New replication or failover choices require an ADR-level decision.
