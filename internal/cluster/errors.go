package cluster

import "errors"

var (
	ErrStaleTerm          = errors.New("stale term")
	ErrNotLeader          = errors.New("not the leader for this slot")
	ErrElectionInProgress = errors.New("election already in progress")
	ErrNoCandidate        = errors.New("no candidate available for election")
)
