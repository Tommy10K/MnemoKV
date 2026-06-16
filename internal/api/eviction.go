package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mnemokv/mnemokv/internal/engine/eviction"
)

type evictionPolicyRequest struct {
	Policy string `json:"policy"`
}

type evictionPolicyResponse struct {
	Policy string `json:"policy"`
}

var validPolicies = map[string]struct{}{
	"noeviction": {},
	"fifo":        {},
	"lru":         {},
	"lfu":         {},
	"random":      {},
}

func (s *Server) handleEvictionPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req evictionPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	name := strings.ToLower(strings.TrimSpace(req.Policy))
	if _, ok := validPolicies[name]; !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "unknown policy: " + req.Policy,
		})
		return
	}
	s.engine.Eviction().SetPolicy(eviction.PolicyByName(name))
	writeJSON(w, http.StatusOK, evictionPolicyResponse{Policy: name})
}
