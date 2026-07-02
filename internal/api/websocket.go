package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			payload := s.snapshotPayload()
			data, err := json.Marshal(payload)
			if err != nil {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) snapshotPayload() map[string]any {
	mem := s.engine.Memory()
	out := map[string]any{
		"timestamp":      time.Now().Unix(),
		"usedBytes":      mem.Used(),
		"memoryLimit":    mem.Limit(),
		"availableBytes": apiAvailableBytes(mem),
		"policy":         s.engine.Eviction().Policy().Name(),
	}
	if s.metrics != nil {
		out["counters"] = s.metrics.Snapshot()
		out["rejectedWrites"] = s.metrics.Counter("eviction.rejected_writes")
	}
	if s.controllerStatus != nil {
		out["recovery"] = s.controllerStatus.StatusSnapshot()
	}
	return out
}
