package cluster

import "time"

type ReplicationRecord struct {
	SourceNodeID string
	Slot         uint16
	Term         uint64
	Sequence     uint64
	Args         []string
	Timestamp    time.Time
}
