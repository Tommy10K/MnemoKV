package api

import "net/http"

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/engine/state", s.handleEngineState)
	mux.HandleFunc("/metrics/summary", s.handleMetricsSummary)
	mux.HandleFunc("/cluster/state", s.handleClusterState)
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/commands", s.handleCommands)
}
