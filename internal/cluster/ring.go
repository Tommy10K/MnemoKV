package cluster

import (
	"hash/fnv"
	"sort"
	"strconv"
)

const defaultVirtualNodes = 64

type Ring struct {
	virtualNodes int
	points       []ringPoint
}

type ringPoint struct {
	hash   uint32
	nodeID string
}

func NewRing(nodes []Node, virtualNodes int) *Ring {
	if virtualNodes <= 0 {
		virtualNodes = defaultVirtualNodes
	}
	r := &Ring{virtualNodes: virtualNodes}
	for _, n := range nodes {
		r.add(n.ID)
	}
	return r
}

func (r *Ring) add(nodeID string) {
	for i := 0; i < r.virtualNodes; i++ {
		h := hash32(nodeID + "#" + strconv.Itoa(i))
		r.points = append(r.points, ringPoint{hash: h, nodeID: nodeID})
	}
	sort.Slice(r.points, func(i, j int) bool {
		return r.points[i].hash < r.points[j].hash
	})
}

func (r *Ring) Owner(key []byte) string {
	if len(r.points) == 0 {
		return ""
	}
	h := hash32Bytes(key)
	idx := sort.Search(len(r.points), func(i int) bool {
		return r.points[i].hash >= h
	})
	if idx == len(r.points) {
		idx = 0
	}
	return r.points[idx].nodeID
}

func (r *Ring) Slot(key []byte) uint16 {
	if len(r.points) == 0 {
		return 0
	}
	return uint16(hash32Bytes(key) & 0x3fff)
}

func hash32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func hash32Bytes(b []byte) uint32 {
	h := fnv.New32a()
	_, _ = h.Write(b)
	return h.Sum32()
}
