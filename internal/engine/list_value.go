package engine

import "sync"

type ListNode struct {
	Prev  *ListNode
	Next  *ListNode
	Value []byte
}

type ListValue struct {
	mu   sync.RWMutex
	head *ListNode
	tail *ListNode
	len  int
}

func NewListValue() *ListValue {
	return &ListValue{}
}

func (l *ListValue) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.len
}

func (l *ListValue) LPush(values ...[]byte) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, v := range values {
		node := &ListNode{Value: v}
		if l.head == nil {
			l.head = node
			l.tail = node
		} else {
			node.Next = l.head
			l.head.Prev = node
			l.head = node
		}
		l.len++
	}
	return l.len
}

func (l *ListValue) RPush(values ...[]byte) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, v := range values {
		node := &ListNode{Value: v}
		if l.tail == nil {
			l.head = node
			l.tail = node
		} else {
			node.Prev = l.tail
			l.tail.Next = node
			l.tail = node
		}
		l.len++
	}
	return l.len
}

func (l *ListValue) LPop() ([]byte, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.head == nil {
		return nil, false
	}
	node := l.head
	l.head = node.Next
	if l.head == nil {
		l.tail = nil
	} else {
		l.head.Prev = nil
	}
	l.len--
	return node.Value, true
}

func (l *ListValue) RPop() ([]byte, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tail == nil {
		return nil, false
	}
	node := l.tail
	l.tail = node.Prev
	if l.tail == nil {
		l.head = nil
	} else {
		l.tail.Next = nil
	}
	l.len--
	return node.Value, true
}

func (l *ListValue) LLen() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.len
}

const listNodeOverhead = 48

func listEntrySize(key string, lv *ListValue) uint64 {
	base := uint64(stringEntryOverhead + len(key))
	lv.mu.RLock()
	defer lv.mu.RUnlock()
	size := base
	node := lv.head
	for node != nil {
		size += listNodeOverhead + uint64(len(node.Value))
		node = node.Next
	}
	return size
}
