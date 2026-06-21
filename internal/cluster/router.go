package cluster

type Router struct {
	localNodeID string
	metadata    *Metadata
}

func NewRouter(localNodeID string, metadata *Metadata) *Router {
	return &Router{localNodeID: localNodeID, metadata: metadata}
}

func (r *Router) Resolve(key []byte) Route {
	if r == nil || r.metadata == nil {
		return Route{OwnerNodeID: r.localNodeID, IsLocal: true}
	}
	slot := r.metadata.SlotForKey(key)
	state, ok := r.metadata.Slot(slot)
	if !ok || state.LeaderID == "" {
		return Route{Slot: slot}
	}
	return Route{OwnerNodeID: state.LeaderID, Slot: slot, IsLocal: state.LeaderID == r.localNodeID}
}

func (r *Router) LocalNodeID() string { return r.localNodeID }
