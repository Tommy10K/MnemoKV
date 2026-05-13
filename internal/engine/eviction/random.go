package eviction

type Random struct{}

func (Random) Name() string { return "random" }

func (Random) PickVictims(bytesNeeded uint64, candidates []Candidate) []Candidate {
	var freed uint64
	var victims []Candidate
	for i := range candidates {
		if freed >= bytesNeeded {
			break
		}
		victims = append(victims, candidates[i])
		freed += candidates[i].SizeBytes
	}
	return victims
}
