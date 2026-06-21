package engine

import (
	"github.com/mnemokv/mnemokv/internal/resp"
)

func (x *Executor) cmdLPush(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 2 {
		return wrongArgs("lpush")
	}
	key := cmd.Args[0]
	values := cmd.Args[1:]

	newLen, err := x.store.listPush(key, values, true)
	if err != nil {
		return wrongTypeError()
	}
	return resp.Integer(int64(newLen))
}

func (x *Executor) cmdRPush(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) < 2 {
		return wrongArgs("rpush")
	}
	key := cmd.Args[0]
	values := cmd.Args[1:]

	newLen, err := x.store.listPush(key, values, false)
	if err != nil {
		return wrongTypeError()
	}
	return resp.Integer(int64(newLen))
}

func (x *Executor) cmdLPop(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("lpop")
	}
	val, ok, err := x.store.listPop(cmd.Args[0], true)
	if err != nil {
		return wrongTypeError()
	}
	if !ok {
		return resp.NullBulk
	}
	return resp.BulkBytes(val)
}

func (x *Executor) cmdRPop(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("rpop")
	}
	val, ok, err := x.store.listPop(cmd.Args[0], false)
	if err != nil {
		return wrongTypeError()
	}
	if !ok {
		return resp.NullBulk
	}
	return resp.BulkBytes(val)
}

func (x *Executor) cmdLLen(cmd *resp.Command) resp.Frame {
	if len(cmd.Args) != 1 {
		return wrongArgs("llen")
	}
	key := cmd.Args[0]
	e, ok := x.store.Peek(key)
	if !ok {
		return resp.Integer(0)
	}
	if e.Type != ValueTypeList {
		return wrongTypeError()
	}
	lv := e.Value.(*ListValue)
	return resp.Integer(int64(lv.LLen()))
}

func wrongTypeError() resp.Frame {
	return resp.NewError("WRONGTYPE", "Operation against a key holding the wrong kind of value")
}

// listPush operates directly under the stripe lock to avoid the deadlock that
// would occur if WithEntry tried to call Put.
func (s *Store) listPush(key []byte, values [][]byte, left bool) (int, error) {
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
		if e.Type != ValueTypeList {
			return 0, ErrWrongType
		}
		lv := e.Value.(*ListValue)
		var newLen int
		if left {
			newLen = lv.LPush(values...)
		} else {
			newLen = lv.RPush(values...)
		}
		oldSize := e.SizeBytes
		e.SizeBytes = listEntrySize(e.Key, lv)
		e.touchWrite(now)
		s.adjustUsed(oldSize, e.SizeBytes)
		return newLen, nil
	}

	lv := NewListValue()
	var newLen int
	if left {
		newLen = lv.LPush(values...)
	} else {
		newLen = lv.RPush(values...)
	}
	ne := &Entry{
		Key:          string(key),
		Type:         ValueTypeList,
		Value:        lv,
		SizeBytes:    listEntrySize(string(key), lv),
		CreatedAtNs:  now,
		UpdatedAtNs:  now,
		LastAccessNs: now,
		AccessCount:  1,
		Version:      1,
	}
	st.entries[ne.Key] = ne
	s.addUsed(ne.SizeBytes)
	return newLen, nil
}

// listPop pops from left or right under the stripe lock.
func (s *Store) listPop(key []byte, left bool) ([]byte, bool, error) {
	st := s.stripeFor(key)
	now := nowNanos()
	st.mu.Lock()
	defer st.mu.Unlock()

	e, ok := st.entries[string(key)]
	if !ok {
		return nil, false, nil
	}
	if e.IsExpired(now) {
		delete(st.entries, e.Key)
		s.subUsed(e.SizeBytes)
		return nil, false, nil
	}
	if e.Type != ValueTypeList {
		return nil, false, ErrWrongType
	}

	lv := e.Value.(*ListValue)
	var val []byte
	var popped bool
	if left {
		val, popped = lv.LPop()
	} else {
		val, popped = lv.RPop()
	}
	if !popped {
		return nil, false, nil
	}

	if lv.LLen() == 0 {
		delete(st.entries, e.Key)
		s.subUsed(e.SizeBytes)
	} else {
		oldSize := e.SizeBytes
		e.SizeBytes = listEntrySize(e.Key, lv)
		e.touchWrite(now)
		s.adjustUsed(oldSize, e.SizeBytes)
	}
	return val, true, nil
}
