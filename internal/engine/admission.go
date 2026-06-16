package engine

import (
	"context"

	"github.com/mnemokv/mnemokv/internal/resp"
)

const maxEvictionPasses = 64

var oomFrame = resp.NewError("OOM", "command not allowed when used memory would exceed maxmemory")

type admissionPlan struct {
	key            string
	willWrite      bool
	requiredGrowth uint64
	resultingSize  uint64
	response       resp.Frame
}

func (e *Engine) executeWithAdmission(cmd *resp.Command) resp.Frame {
	e.admissionMu.Lock()
	defer e.admissionMu.Unlock()

	plan := e.planAdmission(cmd)
	if plan.response != nil {
		if isOOMFrame(plan.response) {
			e.metrics.IncCounter("eviction.rejected_writes")
		}
		return plan.response
	}

	if plan.willWrite && e.memory.HasLimit() && plan.requiredGrowth > 0 {
		if plan.resultingSize > e.memory.Limit() {
			e.metrics.IncCounter("eviction.rejected_writes")
			return oomFrame
		}
		if frame := e.freeForAdmission(context.Background(), plan); frame != nil {
			e.metrics.IncCounter("eviction.rejected_writes")
			return frame
		}
	}

	if e.writeHook != nil && e.hookSync {
		if err := e.writeHook(context.Background(), cmd); err != nil {
			return resp.NewError("ERR", "replication failed: "+err.Error())
		}
	}

	frame := e.executor.Execute(cmd)
	if e.writeHook != nil && !e.hookSync && !isErrorFrame(frame) {
		_ = e.writeHook(context.Background(), cmd)
	}
	return frame
}

func (e *Engine) planAdmission(cmd *resp.Command) admissionPlan {
	switch cmd.Name {
	case "SET":
		return e.planSet(cmd)
	case "INCR":
		return e.planIncr(cmd)
	case "LPUSH", "RPUSH":
		return e.planListPush(cmd)
	case "ZADD":
		return e.planZAdd(cmd)
	default:
		return admissionPlan{willWrite: IsWriteCommand(cmd.Name)}
	}
}

func (e *Engine) planSet(cmd *resp.Command) admissionPlan {
	if len(cmd.Args) < 2 {
		return admissionPlan{response: wrongArgs("set")}
	}
	opts, frame := parseSetOptions(cmd.Args[2:])
	if frame != nil {
		return admissionPlan{response: frame}
	}

	key := cmd.Args[0]
	entry, exists := e.store.Peek(key)
	if opts.condition == setIfMissing && exists {
		return admissionPlan{response: resp.NullBulk}
	}
	if opts.condition == setIfPresent && !exists {
		return admissionPlan{response: resp.NullBulk}
	}

	newSize := stringEntrySize(key, cmd.Args[1])
	var oldSize uint64
	if exists {
		oldSize = entry.SizeBytes
	}
	return admissionPlan{
		key:            string(key),
		willWrite:      true,
		requiredGrowth: positiveDelta(oldSize, newSize),
		resultingSize:  newSize,
	}
}

func (e *Engine) planIncr(cmd *resp.Command) admissionPlan {
	if len(cmd.Args) != 1 {
		return admissionPlan{response: wrongArgs("incr")}
	}

	key := cmd.Args[0]
	entry, exists := e.store.Peek(key)
	var current int64
	var oldSize uint64
	if exists {
		if entry.Type != ValueTypeString {
			return admissionPlan{response: wrongTypeError()}
		}
		sv, _ := entry.Value.(*StringValue)
		if sv == nil {
			return admissionPlan{response: wrongTypeError()}
		}
		parsed, ok := parseInt64(sv.Data)
		if !ok {
			return admissionPlan{response: resp.NewError("ERR", "value is not an integer or out of range")}
		}
		current = parsed
		oldSize = entry.SizeBytes
	}
	if current == maxInt64 {
		return admissionPlan{response: resp.NewError("ERR", "increment or decrement would overflow")}
	}

	newBytes := formatInt64(current + 1)
	newSize := stringEntrySize(key, newBytes)
	return admissionPlan{
		key:            string(key),
		willWrite:      true,
		requiredGrowth: positiveDelta(oldSize, newSize),
		resultingSize:  newSize,
	}
}

func (e *Engine) planListPush(cmd *resp.Command) admissionPlan {
	if len(cmd.Args) < 2 {
		return admissionPlan{response: wrongArgs(lowerCommand(cmd.Name))}
	}

	key := cmd.Args[0]
	entry, exists := e.store.Peek(key)
	var oldSize uint64
	if exists {
		if entry.Type != ValueTypeList {
			return admissionPlan{response: wrongTypeError()}
		}
		oldSize = entry.SizeBytes
	}

	delta := pushedListBytes(cmd.Args[1:])
	newSize := delta
	if exists {
		newSize = saturatingAdd(oldSize, delta)
	} else {
		newSize = saturatingAdd(uint64(stringEntryOverhead+len(key)), delta)
	}
	return admissionPlan{
		key:            string(key),
		willWrite:      true,
		requiredGrowth: positiveDelta(oldSize, newSize),
		resultingSize:  newSize,
	}
}

func (e *Engine) planZAdd(cmd *resp.Command) admissionPlan {
	if len(cmd.Args) < 3 || len(cmd.Args[1:])%2 != 0 {
		return admissionPlan{response: wrongArgs("zadd")}
	}
	pairs, frame := parseZAddPairs(cmd.Args[1:])
	if frame != nil {
		return admissionPlan{response: frame}
	}

	key := cmd.Args[0]
	entry, exists := e.store.Peek(key)
	var (
		oldSize uint64
		zv      *ZSetValue
	)
	if exists {
		if entry.Type != ValueTypeZSet {
			return admissionPlan{response: wrongTypeError()}
		}
		oldSize = entry.SizeBytes
		zv, _ = entry.Value.(*ZSetValue)
		if zv == nil {
			return admissionPlan{response: wrongTypeError()}
		}
	}

	addedBytes := uint64(0)
	newMembers := make(map[string]struct{}, len(pairs))
	for _, pair := range pairs {
		if _, seen := newMembers[pair.member]; seen {
			continue
		}
		if zv != nil {
			if _, exists := zv.Score(pair.member); exists {
				continue
			}
		}
		newMembers[pair.member] = struct{}{}
		addedBytes = saturatingAdd(addedBytes, uint64(64+len(pair.member)))
	}

	newSize := oldSize
	if exists {
		newSize = saturatingAdd(oldSize, addedBytes)
	} else {
		newSize = saturatingAdd(uint64(zsetEntryOverhead+len(key)), addedBytes)
	}
	return admissionPlan{
		key:            string(key),
		willWrite:      true,
		requiredGrowth: positiveDelta(oldSize, newSize),
		resultingSize:  newSize,
	}
}

func (e *Engine) freeForAdmission(ctx context.Context, plan admissionPlan) resp.Frame {
	for pass := 0; pass < maxEvictionPasses; pass++ {
		available := e.memory.Available()
		if available >= plan.requiredGrowth {
			return nil
		}

		policy := e.eviction.Policy()
		if policy.Name() == "noeviction" {
			return oomFrame
		}

		needed := plan.requiredGrowth - available
		e.metrics.IncCounter("eviction.attempts")
		victims := e.eviction.PickVictims(needed, plan.key)
		if len(victims) == 0 {
			return oomFrame
		}

		var progress bool
		for _, victim := range victims {
			if victim.Key == plan.key {
				continue
			}
			if e.writeHook != nil && e.hookSync {
				if err := e.writeHook(ctx, evictionDeleteCommand(victim.Key)); err != nil {
					return resp.NewError("ERR", "replication failed: "+err.Error())
				}
			}
			freed, ok := e.store.DeleteWithSize([]byte(victim.Key))
			if !ok {
				continue
			}
			progress = true
			e.metrics.IncCounter("eviction.keys_evicted")
			e.metrics.IncCounter("eviction.count")
			e.metrics.AddCounter("eviction.bytes_freed", freed)
			if e.writeHook != nil && !e.hookSync {
				_ = e.writeHook(ctx, evictionDeleteCommand(victim.Key))
			}
		}
		if !progress {
			return oomFrame
		}
	}
	return oomFrame
}

func evictionDeleteCommand(key string) *resp.Command {
	return &resp.Command{Name: "DEL", Args: [][]byte{[]byte(key)}}
}

func positiveDelta(oldSize, newSize uint64) uint64 {
	if newSize <= oldSize {
		return 0
	}
	return newSize - oldSize
}

func pushedListBytes(values [][]byte) uint64 {
	var size uint64
	for _, value := range values {
		size = saturatingAdd(size, uint64(listNodeOverhead+len(value)))
	}
	return size
}

func saturatingAdd(a, b uint64) uint64 {
	if ^uint64(0)-a < b {
		return ^uint64(0)
	}
	return a + b
}

func lowerCommand(name string) string {
	switch name {
	case "LPUSH":
		return "lpush"
	case "RPUSH":
		return "rpush"
	default:
		return name
	}
}

func isOOMFrame(f resp.Frame) bool {
	err, ok := f.(resp.Error)
	return ok && err.Prefix == oomFrame.Prefix && err.Message == oomFrame.Message
}
