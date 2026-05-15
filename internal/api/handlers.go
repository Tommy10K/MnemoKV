package api

import (
	"net/http"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status: "ok",
		NodeID: s.node.ID,
		Mode:   s.node.Mode,
	})
}

func (s *Server) handleEngineState(w http.ResponseWriter, r *http.Request) {
	mem := s.engine.Memory()
	policy := s.engine.Eviction().Policy().Name()
	writeJSON(w, http.StatusOK, EngineStateResponse{
		UsedBytes:      mem.Used(),
		MemoryLimit:    mem.Limit(),
		UsageRatio:     mem.UsageRatio(),
		EvictionPolicy: policy,
	})
}

func (s *Server) handleMetricsSummary(w http.ResponseWriter, r *http.Request) {
	if s.metrics == nil {
		writeJSON(w, http.StatusOK, MetricsSummary{Counters: map[string]uint64{}})
		return
	}
	writeJSON(w, http.StatusOK, MetricsSummary{Counters: s.metrics.Snapshot()})
}

func (s *Server) handleClusterState(w http.ResponseWriter, r *http.Request) {
	resp := ClusterStateResponse{
		Enabled:   s.cluster.Enabled,
		NodeID:    s.node.ID,
		WriteMode: s.cluster.WriteSafetyMode,
	}
	for _, p := range s.cluster.Peers {
		resp.Peers = append(resp.Peers, p.ID)
	}
	if s.cluMgr != nil {
		for _, m := range s.cluMgr.Membership() {
			resp.Membership = append(resp.Membership, PeerStatus{
				ID:      m.ID,
				Address: m.Address,
				State:   m.State,
			})
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
