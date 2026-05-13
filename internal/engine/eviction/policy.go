package eviction

type Candidate struct {
	Key          string
	SizeBytes    uint64
	CreatedAtNs  int64
	LastAccessNs int64
	AccessCount  uint32
}

type Policy interface {
	Name() string
	PickVictims(bytesNeeded uint64, candidates []Candidate) []Candidate
}
