package api

import (
	"net/http"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	dataState := "active"
	if s.cluMgr != nil {
		dataState = s.cluMgr.DataState()
	}
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		NodeID:    s.node.ID,
		Mode:      s.node.Mode,
		DataState: dataState,
	})
}

func (s *Server) handleEngineState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	mem := s.engine.Memory()
	policy := s.engine.Eviction().Policy().Name()
	var rejected uint64
	if s.metrics != nil {
		rejected = s.metrics.Counter("eviction.rejected_writes")
	}
	writeJSON(w, http.StatusOK, EngineStateResponse{
		UsedBytes:      mem.Used(),
		MemoryLimit:    mem.Limit(),
		AvailableBytes: mem.Available(),
		UsageRatio:     mem.UsageRatio(),
		EvictionPolicy: policy,
		RejectedWrites: rejected,
	})
}

func (s *Server) handleMetricsSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.metrics == nil {
		writeJSON(w, http.StatusOK, MetricsSummary{Counters: map[string]uint64{}, Gauges: map[string]float64{}})
		return
	}
	writeJSON(w, http.StatusOK, MetricsSummary{Counters: s.metrics.Snapshot(), Gauges: s.metrics.GaugesSnapshot()})
}

func (s *Server) handleClusterState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	resp := ClusterStateResponse{
		Enabled:      s.cluster.Enabled,
		NodeID:       s.node.ID,
		ClusterID:    s.cluster.ID,
		SlotCount:    s.cluster.SlotCount,
		RoutingMode:  s.cluster.RoutingMode,
		FailoverMode: s.cluster.FailoverMode,
		DataState:    "active",
	}
	for _, p := range s.cluster.Peers {
		resp.Peers = append(resp.Peers, p.ID)
	}
	if s.cluMgr != nil {
		resp.DataState = s.cluMgr.DataState()
		for _, m := range s.cluMgr.Membership() {
			resp.Membership = append(resp.Membership, PeerStatus{
				ID:      m.ID,
				Address: m.Address,
				State:   m.State,
			})
		}
		if metadata := s.cluMgr.Metadata(); metadata != nil {
			state := metadata.Snapshot()
			resp.MetadataVersion = state.Version
			resp.Slots = make([]SlotStatus, len(state.Slots))
			for i, slot := range state.Slots {
				resp.Slots[i] = slotStatus(metadata, slot)
			}
		}
	}
	if s.controllerStatus != nil {
		status := s.controllerStatus.StatusSnapshot()
		resp.Recovery = &status
	}
	writeJSON(w, http.StatusOK, resp)
}
