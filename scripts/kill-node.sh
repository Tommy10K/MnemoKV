#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$ROOT_DIR/bin"
PID_FILE="$BIN_DIR/cluster.pids"

if [[ $# -lt 1 ]]; then
	echo "Usage: $0 <node-number: 1|2|3>" >&2
	exit 1
fi

NODE_NUM="$1"
if [[ ! -f "$PID_FILE" ]]; then
	echo "No cluster.pids file found. Is the cluster running?" >&2
	exit 1
fi

read -r PID1 PID2 PID3 < "$PID_FILE"

case "$NODE_NUM" in
	1) PID=$PID1 ;;
	2) PID=$PID2 ;;
	3) PID=$PID3 ;;
	*) echo "Invalid node number: $NODE_NUM (use 1, 2, or 3)" >&2; exit 1 ;;
esac

echo "Killing node-$NODE_NUM (PID $PID)..."
kill "$PID" 2>/dev/null || echo "Process $PID already dead"
