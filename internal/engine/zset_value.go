package engine

import "sync"

type ZSetMember struct {
	Member string
	Score  float64
}

type ZSetValue struct {
	mu    sync.RWMutex
	index map[string]float64
	list  *SkipList
}

func NewZSetValue() *ZSetValue {
	return &ZSetValue{
		index: make(map[string]float64),
		list:  NewSkipList(),
	}
}

// Add inserts or updates a member. Returns true if the member was newly added.
func (z *ZSetValue) Add(score float64, member string) bool {
	z.mu.Lock()
	defer z.mu.Unlock()

	if oldScore, exists := z.index[member]; exists {
		if oldScore == score {
			return false
		}
		z.list.Delete(oldScore, member)
		z.list.Insert(score, member)
		z.index[member] = score
		return false
	}

	z.list.Insert(score, member)
	z.index[member] = score
	return true
}

// Range returns members from rank start to stop inclusive (0-based, supports negatives).
func (z *ZSetValue) Range(start, stop int) []ZSetMember {
	z.mu.RLock()
	defer z.mu.RUnlock()

	nodes := z.list.Range(start, stop)
	result := make([]ZSetMember, len(nodes))
	for i, n := range nodes {
		result[i] = ZSetMember{Member: n.Member, Score: n.Score}
	}
	return result
}

func (z *ZSetValue) Card() int {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return z.list.Len()
}

func (z *ZSetValue) Score(member string) (float64, bool) {
	z.mu.RLock()
	defer z.mu.RUnlock()
	s, ok := z.index[member]
	return s, ok
}

const zsetEntryOverhead = 80

func zsetEntrySize(key string, z *ZSetValue) uint64 {
	base := uint64(zsetEntryOverhead + len(key))
	z.mu.RLock()
	defer z.mu.RUnlock()
	for member := range z.index {
		base += 64 + uint64(len(member)) // node + map entry estimate
	}
	return base
}
