package cluster

import (
	"context"
	"strings"

	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func (m *Manager) AttachEngine(eng *engine.Engine) {
	if m.replicator == nil || eng == nil {
		return
	}
	mode := strings.ToLower(m.cfg.WriteSafetyMode)
	switch mode {
	case "strong":
		eng.SetWriteHook(m.strongHook(), true)
	default:
		eng.SetWriteHook(m.asyncHook(), false)
	}
}

func (m *Manager) asyncHook() engine.WriteHook {
	return func(_ context.Context, cmd *resp.Command) error {
		if err := m.fenceCheck(cmd); err != nil {
			return err
		}
		args := commandToStrings(cmd)
		m.replicator.Replicate(args, slotForCommand(m, cmd))
		return nil
	}
}

func (m *Manager) strongHook() engine.WriteHook {
	return func(ctx context.Context, cmd *resp.Command) error {
		if err := m.fenceCheck(cmd); err != nil {
			return err
		}
		args := commandToStrings(cmd)
		return m.replicator.ReplicateSync(ctx, args, slotForCommand(m, cmd))
	}
}

func (m *Manager) fenceCheck(cmd *resp.Command) error {
	if m.controlPlane == nil {
		return nil
	}
	slot := slotForCommand(m, cmd)
	return m.controlPlane.ValidateWriteTerm(slot, m.controlPlane.CurrentTerm())
}

func commandToStrings(cmd *resp.Command) []string {
	out := make([]string, 0, len(cmd.Args)+1)
	out = append(out, cmd.Name)
	for _, a := range cmd.Args {
		out = append(out, string(a))
	}
	return out
}

func slotForCommand(m *Manager, cmd *resp.Command) uint16 {
	if m.ring == nil || len(cmd.Args) == 0 {
		return 0
	}
	return m.ring.Slot(cmd.Args[0])
}
