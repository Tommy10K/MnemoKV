package eviction

import "sort"

type FIFO struct{}

func (FIFO) Name() string { return "fifo" }

func (FIFO) PickVictims(bytesNeeded uint64, candidates []Candidate) []Candidate {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreatedAtNs < candidates[j].CreatedAtNs
	})
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
