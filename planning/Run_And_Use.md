# Run And Use MnemoKV

Last reviewed against the codebase: June 21, 2026.

## Requirements

- Go 1.22 or newer.
- Node.js and npm for the optional frontend.
- PowerShell 7 is recommended for the bundled Windows demos.
- `redis-cli` is optional.

## Standalone Node

From the repository root:

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

The default RESP endpoint is `127.0.0.1:6380`; the HTTP API is `http://127.0.0.1:7380`.

```powershell
redis-cli -p 6380 SET greeting hello
redis-cli -p 6380 GET greeting

Invoke-RestMethod -Method Post `
  -Uri http://127.0.0.1:7380/commands `
  -ContentType application/json `
  -Body '{"args":["GET","greeting"]}'
```

## Frontend

Keep the node running and use another terminal:

```powershell
cd frontend
npm.cmd ci
npm.cmd run dev
```

Open the Vite URL. The initial API target is `http://127.0.0.1:7380`; change it from the Use
section when connecting elsewhere. The selection is persisted. Set `VITE_API_BASE_URL` before
startup to change the initial value.

Useful routes include `/learn`, `/use`, `/use/dashboard`, `/use/console`, `/use/workloads`,
`/use/cluster`, `/use/eviction`, and `/use/benchmarks`. The frontend validates HTTP and SSE
payloads at runtime, so an unexpected response is reported separately from an offline node.

## Reproducible Presentation Data

With the standalone node running:

```powershell
./scripts/load-demo-dataset.ps1
```

This loads `examples/demo-dataset.json` and verifies deterministic string, list, and sorted-set
values. The loader accepts `-ApiBaseUrl` and `-Dataset` overrides.

Other demos:

```powershell
./scripts/demo-standalone.ps1
go run ./cmd/node -config configs/standalone-low-memory.yaml
./scripts/demo-low-memory.ps1
./scripts/demo-persistence.ps1
./scripts/demo-cluster.ps1
```

Snapshot persistence is not a write-ahead log. Cluster failover is manual: promote the surviving
assigned replica, assign a replacement, then transfer a full slot snapshot.

## Experimental Automatic Recovery

The five `configs/cluster-node-{1..5}-auto.yaml` presets enable the embedded Raft controller. Manual
mode remains the default. Automatic recovery preserves one leader and one synchronous replica per
slot, so its data guarantee is **one node failure at a time with repair in between**. A second
destructive failure before repair completes can remove the last reachable copy of a slot; that slot
is reported as `potential_data_loss` and is never recreated empty. Unaffected slots continue serving,
and a returning node's old application data is not trusted or merged in v1.

## Benchmark Import

The Benchmarks page accepts raw `go test -bench -benchmem` output or JSON. Use **Load built-in
example** for a deterministic presentation without running a benchmark.

## Verify Changes

```powershell
go test ./...
go test -race ./...
go vet ./...

cd frontend
npm.cmd ci
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

The Edge suite starts and cleans up isolated backend and Vite processes. It tests live command and
dashboard behavior, offline and malformed responses, API switching, responsive layouts, keyboard
behavior, reduced motion, and WCAG A/AA rules.

## Common Problems

- If the UI is offline, confirm `/health` works at the selected API URL and that no old process owns
  the configured ports.
- Under a restrictive PowerShell execution policy, use an allowed PowerShell 7 session or an
  approved `-ExecutionPolicy Bypass` invocation for local scripts.
- If `npm ci` fails after dependency edits, regenerate and commit `package-lock.json`; do not rely
  on undeclared `--no-save` packages.
- Strict cluster replication rejects writes when the assigned replica is unavailable or not ready.
