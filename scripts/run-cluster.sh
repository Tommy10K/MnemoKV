#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$ROOT_DIR/bin"

mkdir -p "$BIN_DIR"

echo "Building mnemokv-node..."
go build -o "$BIN_DIR/mnemokv-node" "$ROOT_DIR/cmd/node"

echo "Starting 3-node cluster..."
"$BIN_DIR/mnemokv-node" -config "$ROOT_DIR/configs/cluster-node-1.yaml" &
PID1=$!
"$BIN_DIR/mnemokv-node" -config "$ROOT_DIR/configs/cluster-node-2.yaml" &
PID2=$!
"$BIN_DIR/mnemokv-node" -config "$ROOT_DIR/configs/cluster-node-3.yaml" &
PID3=$!

echo "PIDs: node-1=$PID1 node-2=$PID2 node-3=$PID3"
echo "$PID1 $PID2 $PID3" > "$BIN_DIR/cluster.pids"

sleep 1
echo "Cluster running. Use scripts/kill-node.sh <1|2|3> to stop a node."
echo "Stop all: kill $PID1 $PID2 $PID3"
wait
