#!/usr/bin/env bash
# Quick black-box smoke test against a running node using redis-cli.
# Usage: PORT=6380 ./scripts/smoke-test.sh
set -euo pipefail

PORT="${PORT:-6380}"
HOST="${HOST:-127.0.0.1}"
CLI="${REDIS_CLI:-redis-cli}"

if ! command -v "$CLI" >/dev/null 2>&1; then
    echo "redis-cli not found in PATH; install it or set REDIS_CLI" >&2
    exit 2
fi

run() {
    local expected="$1"; shift
    local got
    got="$("$CLI" -h "$HOST" -p "$PORT" "$@")"
    if [[ "$got" != "$expected" ]]; then
        echo "FAIL: $* -> '$got' (expected '$expected')" >&2
        exit 1
    fi
    echo "ok: $*"
}

run "PONG"   PING
run "OK"     SET foo bar
run "bar"    GET foo
run "1"      EXISTS foo
run "1"      DEL foo
run "0"      EXISTS foo
run "1"      INCR counter
run "2"      INCR counter
run "OK"     SET ttlkey v EX 100
ttl="$("$CLI" -h "$HOST" -p "$PORT" TTL ttlkey)"
if (( ttl <= 0 || ttl > 100 )); then
    echo "FAIL: TTL ttlkey -> $ttl" >&2
    exit 1
fi
echo "ok: TTL ttlkey -> $ttl"
run "1"      EXPIRE ttlkey 50
run "OK"     FLUSHDB
echo "smoke test passed"
