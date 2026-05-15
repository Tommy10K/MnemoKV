package cluster

import "testing"

func TestRingDeterministicOwner(t *testing.T) {
	nodes := []Node{
		{ID: "a", Address: "a:1"},
		{ID: "b", Address: "b:1"},
		{ID: "c", Address: "c:1"},
	}
	r := NewRing(nodes, 32)
	owners := make(map[string]int)
	for i := 0; i < 1000; i++ {
		key := []byte("key:")
		key = append(key, byte(i&0xff), byte((i>>8)&0xff))
		owners[r.Owner(key)]++
	}
	for _, n := range nodes {
		if owners[n.ID] == 0 {
			t.Errorf("node %s never selected", n.ID)
		}
	}
}

func TestRingStable(t *testing.T) {
	r1 := NewRing([]Node{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 16)
	r2 := NewRing([]Node{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 16)
	for i := 0; i < 100; i++ {
		key := []byte{byte(i)}
		if r1.Owner(key) != r2.Owner(key) {
			t.Fatalf("owner mismatch for key %d", i)
		}
	}
}

func TestRouterLocalDetection(t *testing.T) {
	ring := NewRing([]Node{{ID: "n1"}, {ID: "n2"}}, 16)
	router := NewRouter("n1", ring)

	saw := map[bool]int{}
	for i := 0; i < 100; i++ {
		route := router.Resolve([]byte{byte(i)})
		saw[route.IsLocal]++
	}
	if saw[true] == 0 || saw[false] == 0 {
		t.Fatalf("expected mix of local/remote, got %v", saw)
	}
}

func TestRouterEmptyRing(t *testing.T) {
	router := NewRouter("n1", nil)
	route := router.Resolve([]byte("k"))
	if !route.IsLocal || route.OwnerNodeID != "n1" {
		t.Fatalf("expected local fallback, got %+v", route)
	}
}
