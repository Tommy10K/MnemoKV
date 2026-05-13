package cluster

import (
	"context"

	"github.com/mnemokv/mnemokv/internal/resp"
)

type Transport interface {
	Forward(ctx context.Context, nodeID string, cmd *resp.Command) (resp.Frame, error)
	SendReplication(ctx context.Context, nodeID string, rec ReplicationRecord) error
	Close() error
}
