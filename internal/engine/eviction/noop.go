package eviction

type Noop struct{}

func (Noop) Name() string { return "noop" }

func (Noop) PickVictims(_ uint64, _ []Candidate) []Candidate { return nil }
