package api

import (
	"errors"
	"net/http"

	"github.com/mnemokv/mnemokv/internal/persistence"
)

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	if s.snapshots == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": persistence.ErrDisabled.Error()})
		return
	}
	result, err := s.snapshots.Snapshot()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, persistence.ErrDisabled) {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
