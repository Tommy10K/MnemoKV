package eviction

import "sort"

type LFU struct{}

func (LFU) Name() string { return "lfu" }

func (LFU) PickVictims(bytesNeeded uint64, candidates []Candidate) []Candidate {
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].AccessCount < candidates[j].AccessCount
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
