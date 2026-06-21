package cluster

import "errors"

var (
	ErrStaleTerm          = errors.New("stale term")
	ErrNotLeader          = errors.New("not the leader for this slot")
	ErrNotReplica         = errors.New("not the replica for this slot")
	ErrReplicaUnavailable = errors.New("slot replica is unavailable or not synchronized")
	ErrSequenceGap        = errors.New("replication sequence gap")
	ErrSlotOutOfRange     = errors.New("slot is out of range")
	ErrUnknownNode        = errors.New("unknown cluster node")
	ErrReplicaIsLeader    = errors.New("replica cannot be the slot leader")
	ErrClusterMismatch    = errors.New("cluster metadata does not match this cluster")
	ErrStaleMetadata      = errors.New("cluster metadata version is stale")
	ErrNoReplicaAssigned  = errors.New("slot has no assigned replica to promote")
)
