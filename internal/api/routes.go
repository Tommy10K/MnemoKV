package api

import "net/http"

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/engine/state", s.handleEngineState)
	mux.HandleFunc("/metrics/summary", s.handleMetricsSummary)
	mux.HandleFunc("/cluster/state", s.handleClusterState)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/commands", s.handleCommands)
	mux.HandleFunc("/engine/eviction-policy", s.handleEvictionPolicy)
	mux.HandleFunc("/admin/snapshot", s.handleSnapshot)
	mux.HandleFunc("/cluster/promote", s.handleClusterPromote)
	mux.HandleFunc("/cluster/replica", s.handleClusterReplica)
	mux.HandleFunc("/cluster/sync", s.handleClusterSync)
	mux.HandleFunc("/cluster/returning/prepare", s.handleReturningNodePrepare)
	mux.HandleFunc("/cluster/returning/admit", s.handleReturningNodeAdmit)
}
