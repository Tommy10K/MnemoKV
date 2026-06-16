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

echo "Generating JSON summary..."
awk '
BEGIN {
	print "["
	first = 1
}
/^Benchmark/ {
	name = $1
	sub(/-[0-9]+$/, "", name)
	ns = "null"
	bytes = "null"
	allocs = "null"
	for (i = 1; i <= NF; i++) {
		if ((i + 1) <= NF && $(i + 1) == "ns/op") ns = $i
		if ((i + 1) <= NF && $(i + 1) == "B/op") bytes = $i
		if ((i + 1) <= NF && $(i + 1) == "allocs/op") allocs = $i
	}
	if (!first) {
		print ","
	}
	printf "  {\"name\":\"%s\",\"nsPerOp\":%s,\"bytesPerOp\":%s,\"allocsPerOp\":%s}", name, ns, bytes, allocs
	first = 0
}
END {
	if (!first) {
		print ""
	}
	print "]"
}
' "$OUTPUT_DIR/engine_bench.txt" > "$OUTPUT_DIR/engine_bench.json"
echo "JSON written to $OUTPUT_DIR/engine_bench.json"
