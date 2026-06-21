package engine

import "math/rand"

const skipListMaxLevel = 32
const skipListP = 0.25

type SkipNode struct {
	Member   string
	Score    float64
	backward *SkipNode
	forward  []*SkipNode
}

type SkipList struct {
	head   *SkipNode
	level  int
	length int
}

func NewSkipList() *SkipList {
	head := &SkipNode{forward: make([]*SkipNode, skipListMaxLevel)}
	return &SkipList{head: head, level: 1}
}

func (sl *SkipList) Len() int { return sl.length }

func (sl *SkipList) Insert(score float64, member string) *SkipNode {
	update := make([]*SkipNode, skipListMaxLevel)
	x := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && (x.forward[i].Score < score ||
			(x.forward[i].Score == score && x.forward[i].Member < member)) {
			x = x.forward[i]
		}
		update[i] = x
	}

	lvl := randomLevel()
	if lvl > sl.level {
		for i := sl.level; i < lvl; i++ {
			update[i] = sl.head
		}
		sl.level = lvl
	}

	node := &SkipNode{
		Member:  member,
		Score:   score,
		forward: make([]*SkipNode, lvl),
	}

	for i := 0; i < lvl; i++ {
		node.forward[i] = update[i].forward[i]
		update[i].forward[i] = node
	}

	if update[0] == sl.head {
		node.backward = nil
	} else {
		node.backward = update[0]
	}
	if node.forward[0] != nil {
		node.forward[0].backward = node
	}

	sl.length++
	return node
}

func (sl *SkipList) Delete(score float64, member string) bool {
	update := make([]*SkipNode, skipListMaxLevel)
	x := sl.head

	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && (x.forward[i].Score < score ||
			(x.forward[i].Score == score && x.forward[i].Member < member)) {
			x = x.forward[i]
		}
		update[i] = x
	}

	x = x.forward[0]
	if x == nil || x.Score != score || x.Member != member {
		return false
	}

	for i := 0; i < sl.level; i++ {
		if update[i].forward[i] != x {
			break
		}
		update[i].forward[i] = x.forward[i]
	}

	if x.forward[0] != nil {
		x.forward[0].backward = x.backward
	}

	for sl.level > 1 && sl.head.forward[sl.level-1] == nil {
		sl.level--
	}
	sl.length--
	return true
}

func (sl *SkipList) Range(start, stop int64) []SkipNode {
	if sl.length == 0 {
		return nil
	}
	length := int64(sl.length)
	if start < 0 {
		start += length
	}
	if stop < 0 {
		stop += length
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		return nil
	}

	x := sl.head
	for i := int64(0); i <= start; i++ {
		x = x.forward[0]
		if x == nil {
			return nil
		}
	}

	result := make([]SkipNode, 0, int(stop-start+1))
	for i := start; i <= stop && x != nil; i++ {
		result = append(result, SkipNode{Member: x.Member, Score: x.Score})
		x = x.forward[0]
	}
	return result
}

func randomLevel() int {
	lvl := 1
	for lvl < skipListMaxLevel && rand.Float64() < skipListP {
		lvl++
	}
	return lvl
}
