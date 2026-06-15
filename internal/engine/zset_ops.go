package engine

import (
	"math"
	"strconv"

	"github.com/mnemokv/mnemokv/internal/resp"
)

func (x *Executor) cmdZAdd(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 3 || len(cmd.Args[1:])%2 != 0 {
		return wrongArgs("zadd")
	}
	key := cmd.Args[0]

	pairs := make([]zsetPair, 0, (len(cmd.Args)-1)/2)
	for i := 1; i < len(cmd.Args); i += 2 {
		score, err := strconv.ParseFloat(string(cmd.Args[i]), 64)
		if err != nil || math.IsNaN(score) {
			return resp.NewError("ERR", "value is not a valid float")
		}
		pairs = append(pairs, zsetPair{score: score, member: string(cmd.Args[i+1])})
	}

	added, err := x.store.zsetAdd(key, pairs)
	if err != nil {
		return wrongTypeError()
	}
	return resp.Integer(int64(added))
}

func (x *Executor) cmdZRange(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 3 && len(cmd.Args) != 4 {
		return wrongArgs("zrange")
	}
	key := cmd.Args[0]
	start, ok1 := parseInt64(cmd.Args[1])
	stop, ok2 := parseInt64(cmd.Args[2])
	if !ok1 || !ok2 {
		return resp.NewError("ERR", "value is not an integer or out of range")
	}

	withScores := false
	if len(cmd.Args) == 4 {
		if upperASCII(cmd.Args[3]) != "WITHSCORES" {
			return resp.NewError("ERR", "syntax error")
		}
		withScores = true
	}

	e, ok := x.store.Get(key)
	if !ok {
		return resp.Array{Items: []resp.Frame{}}
	}
	if e.Type != ValueTypeZSet {
		return wrongTypeError()
	}
	zv := e.Value.(*ZSetValue)
	members := zv.Range(start, stop)

	if withScores {
		items := make([]resp.Frame, 0, len(members)*2)
		for _, m := range members {
			items = append(items, resp.BulkFromString(m.Member))
			items = append(items, resp.BulkFromString(formatFloat64(m.Score)))
		}
		return resp.Array{Items: items}
	}

	items := make([]resp.Frame, 0, len(members))
	for _, m := range members {
		items = append(items, resp.BulkFromString(m.Member))
	}
	return resp.Array{Items: items}
}

func (x *Executor) cmdZCard(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("zcard")
	}
	e, ok := x.store.Get(cmd.Args[0])
	if !ok {
		return resp.Integer(0)
	}
	if e.Type != ValueTypeZSet {
		return wrongTypeError()
	}
	return resp.Integer(int64(e.Value.(*ZSetValue).Card()))
}

func (x *Executor) cmdZScore(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 2 {
		return wrongArgs("zscore")
	}
	e, ok := x.store.Get(cmd.Args[0])
	if !ok {
		return resp.NullBulk
	}
	if e.Type != ValueTypeZSet {
		return wrongTypeError()
	}
	score, exists := e.Value.(*ZSetValue).Score(string(cmd.Args[1]))
	if !exists {
		return resp.NullBulk
	}
	return resp.BulkFromString(formatFloat64(score))
}

func formatFloat64(value float64) string {
	switch {
	case value == 0:
		return "0"
	case math.IsInf(value, 1):
		return "inf"
	case math.IsInf(value, -1):
		return "-inf"
	default:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
}

type zsetPair struct {
	score  float64
	member string
}

func (s *Store) zsetAdd(key []byte, pairs []zsetPair) (int, error) {
	st := s.stripeFor(key)
	now := nowNanos()
	st.mu.Lock()
	defer st.mu.Unlock()

	e, ok := st.entries[string(key)]
	if ok && e.IsExpired(now) {
		delete(st.entries, e.Key)
		s.subUsed(e.SizeBytes)
		ok = false
		e = nil
	}

	if ok {
		if e.Type != ValueTypeZSet {
			return 0, ErrWrongType
		}
		zv := e.Value.(*ZSetValue)
		added := 0
		for _, p := range pairs {
			if zv.Add(p.score, p.member) {
				added++
			}
		}
		oldSize := e.SizeBytes
		e.SizeBytes = zsetEntrySize(e.Key, zv)
		e.touchWrite(now)
		s.adjustUsed(oldSize, e.SizeBytes)
		return added, nil
	}

	zv := NewZSetValue()
	added := 0
	for _, p := range pairs {
		if zv.Add(p.score, p.member) {
			added++
		}
	}
	ne := &Entry{
		Key:          string(key),
		Type:         ValueTypeZSet,
		Value:        zv,
		SizeBytes:    zsetEntrySize(string(key), zv),
		CreatedAtNs:  now,
		UpdatedAtNs:  now,
		LastAccessNs: now,
		AccessCount:  1,
		Version:      1,
	}
	st.entries[ne.Key] = ne
	s.addUsed(ne.SizeBytes)
	return added, nil
}
