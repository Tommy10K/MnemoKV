package eviction

type NoEviction struct{}

func (NoEviction) Name() string { return "noeviction" }

func (NoEviction) PickVictims(_ uint64, _ []Candidate) []Candidate { return nil }
