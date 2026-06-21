package cluster

import (
	"context"

	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/resp"
)

func (m *Manager) AttachEngine(eng *engine.Engine) {
	m.engine = eng
	m.coordinator = NewCoordinator(m, eng)
	if m.replicator == nil || eng == nil {
		return
	}
	eng.SetWriteHook(func(ctx context.Context, cmd *resp.Command) error {
		return m.replicator.Replicate(ctx, cmd)
	}, true)
}

func commandToStrings(cmd *resp.Command) []string {
	out := make([]string, 0, len(cmd.Args)+1)
	out = append(out, cmd.Name)
	for _, arg := range cmd.Args {
		out = append(out, string(arg))
	}
	return out
}
