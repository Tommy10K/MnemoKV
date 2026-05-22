package cluster

import "sync/atomic"

type Term struct {
	value atomic.Uint64
}

func (t *Term) Current() uint64 {
	return t.value.Load()
}

func (t *Term) Advance() uint64 {
	return t.value.Add(1)
}

func (t *Term) Set(v uint64) {
	t.value.Store(v)
}
