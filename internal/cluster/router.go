package cluster

type Router struct {
	localNodeID string
	ring        *Ring
}

func NewRouter(localNodeID string, ring *Ring) *Router {
	return &Router{localNodeID: localNodeID, ring: ring}
}

func (r *Router) Resolve(key []byte) Route {
	if r.ring == nil {
		return Route{OwnerNodeID: r.localNodeID, IsLocal: true}
	}
	owner := r.ring.Owner(key)
	if owner == "" {
		return Route{OwnerNodeID: r.localNodeID, IsLocal: true}
	}
	return Route{
		OwnerNodeID: owner,
		Slot:        r.ring.Slot(key),
		IsLocal:     owner == r.localNodeID,
	}
}

func (r *Router) LocalNodeID() string { return r.localNodeID }
