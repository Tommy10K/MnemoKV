package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/resp"
)

type Coordinator struct {
	manager *Manager
	engine  *engine.Engine
}

func NewCoordinator(manager *Manager, eng *engine.Engine) *Coordinator {
	return &Coordinator{manager: manager, engine: eng}
}

func (c *Coordinator) Execute(cmd *resp.Command) resp.Frame {
	switch cmd.Name {
	case "REPLICATE":
		return c.applyReplication(cmd)
	case "CLUSTERMETA":
		raw, err := c.manager.MetadataJSON()
		if err != nil {
			return clusterFrameError(err)
		}
		return resp.BulkBytes(raw)
	case "CLUSTERAPPLY":
		return c.applyMetadata(cmd)
	case "CLUSTERSNAPSHOT":
		return c.applyShardSnapshot(cmd)
	case "FLUSHDB", "FLUSHALL":
		return resp.NewError("ERR", "global flush commands are not supported in cluster mode")
	}

	keys := resp.ExtractKeys(cmd)
	if len(keys) == 0 {
		return c.engine.Execute(cmd)
	}
	slot := c.manager.metadata.SlotForKey(keys[0])
	for _, key := range keys[1:] {
		if c.manager.metadata.SlotForKey(key) != slot {
			return resp.NewError("CROSSSLOT", "Keys in request don't hash to the same slot")
		}
	}
	state, ok := c.manager.metadata.Slot(slot)
	if !ok || state.LeaderID == "" {
		return resp.NewError("CLUSTERDOWN", "slot has no leader")
	}
	if state.LeaderID != c.manager.nodeID {
		frame, err := c.manager.proxy.Forward(context.Background(), state.LeaderID, cmd)
		if err != nil {
			return resp.NewError("CLUSTERDOWN", "leader unavailable")
		}
		return frame
	}
	return c.engine.Execute(cmd)
}

func (c *Coordinator) applyReplication(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 5 {
		return resp.NewError("ERR", "REPLICATE requires source, slot, term, sequence, and command")
	}
	slot, err := strconv.ParseUint(string(cmd.Args[1]), 10, 32)
	if err != nil {
		return resp.NewError("ERR", "invalid replication slot")
	}
	term, err := strconv.ParseUint(string(cmd.Args[2]), 10, 64)
	if err != nil {
		return resp.NewError("ERR", "invalid replication term")
	}
	sequence, err := strconv.ParseUint(string(cmd.Args[3]), 10, 64)
	if err != nil {
		return resp.NewError("ERR", "invalid replication sequence")
	}
	rec := ReplicationRecord{SourceNodeID: string(cmd.Args[0]), Slot: uint32(slot), Term: term, Sequence: sequence}
	rec.Args = make([]string, len(cmd.Args)-4)
	for i := 4; i < len(cmd.Args); i++ {
		rec.Args[i-4] = string(cmd.Args[i])
	}
	if err := c.manager.ApplyReplication(rec); err != nil {
		return clusterFrameError(err)
	}
	return resp.OK
}

func (c *Coordinator) applyMetadata(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return resp.NewError("ERR", "CLUSTERAPPLY requires one metadata payload")
	}
	var state MetadataSnapshot
	if err := json.Unmarshal(cmd.Args[0], &state); err != nil {
		return resp.NewError("ERR", "invalid cluster metadata")
	}
	if err := c.manager.ApplyMetadata(state); err != nil && !errors.Is(err, ErrStaleMetadata) {
		return clusterFrameError(err)
	}
	return resp.OK
}

func (c *Coordinator) applyShardSnapshot(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return resp.NewError("ERR", "CLUSTERSNAPSHOT requires one snapshot payload")
	}
	var transfer ShardSnapshot
	if err := json.Unmarshal(cmd.Args[0], &transfer); err != nil {
		return resp.NewError("ERR", "invalid shard snapshot")
	}
	if err := c.manager.ApplyShardSnapshot(transfer); err != nil {
		return clusterFrameError(err)
	}
	return resp.OK
}
