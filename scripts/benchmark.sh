#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/results}"

mkdir -p "$OUTPUT_DIR"

echo "Running engine benchmarks..."
go test -bench=. -benchmem -count=3 -timeout 120s "$ROOT_DIR/internal/engine/" | tee "$OUTPUT_DIR/engine_bench.txt"

echo ""
echo "Results written to $OUTPUT_DIR/engine_bench.txt"

if command -v jq >/dev/null 2>&1; then
	echo "Generating JSON summary..."
	grep "^Benchmark" "$OUTPUT_DIR/engine_bench.txt" | awk '{print "{\"name\":\""$1"\",\"ns_op\":"$3",\"allocs_op\":"$5"}"}' > "$OUTPUT_DIR/engine_bench.json"
	echo "JSON written to $OUTPUT_DIR/engine_bench.json"
fi
