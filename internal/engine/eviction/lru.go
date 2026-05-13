package eviction

import "sort"

type LRU struct{}

func (LRU) Name() string { return "lru" }

func (LRU) PickVictims(bytesNeeded uint64, candidates []Candidate) []Candidate {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastAccessNs < candidates[j].LastAccessNs
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
