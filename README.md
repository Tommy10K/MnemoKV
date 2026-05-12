# MnemoKV

An educational, observable, in-memory distributed key/value store written in Go.

This baseline milestone delivers a single-node RESP2 server that is plug-compatible with `redis-cli` for the following commands:

- `PING`
- `ECHO`
- `SET key value [EX sec | PX ms] [NX | XX]`
- `GET`
- `INCR`
- `DEL key [key ...]`
- `EXISTS key [key ...]`
- `EXPIRE key seconds`
- `TTL key`
- `FLUSHDB`
- `COMMAND`, `CLIENT`, `QUIT` (compatibility shims)

Later phases (lists, sorted sets, eviction, observability, clustering, replication, automatic failover) will land on dedicated branches as described in [planning/Backend_Execution_Checklist.md](planning/Backend_Execution_Checklist.md).

## Layout

```
cmd/             entry-point binaries (node, workload, adminctl)
configs/         standalone and cluster YAML configs
docs/adr/        accepted engineering decisions
internal/config  YAML config model and validation
internal/resp    RESP2 parser, writer, command pool
internal/server  TCP listener, per-connection pipeline, graceful shutdown
internal/engine  striped store, executor, string + utility commands
internal/cluster placeholder; populated in later phases
internal/metrics noop sink; populated in later phases
scripts/         smoke and operational scripts
test/integration end-to-end test against a real socket
```

## Build and run

```sh
make build
./bin/mnemokv-node --config configs/standalone.yaml
```

In another terminal:

```sh
redis-cli -p 6380 PING
redis-cli -p 6380 SET foo bar
redis-cli -p 6380 GET foo
```

## Test

```sh
make test           # unit + integration
make race           # race detector
```

The smoke script can drive a running node from a shell:

```sh
PORT=6380 ./scripts/smoke-test.sh
```