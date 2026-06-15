package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mnemokv/mnemokv/internal/resp"
)

type commandRequest struct {
	Args []string `json:"args"`
}

// commandResult mirrors a RESP frame in a shape the browser can render
// directly. Type is one of: "string", "error", "integer", "bulk", "nil",
// "array". Value carries the payload (string for string/error/bulk,
// number for integer, []commandResult for array, nil for nil).
type commandResult struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}
	if len(req.Args) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "args must not be empty"})
		return
	}

	cmd := &resp.Command{Name: strings.ToUpper(req.Args[0])}
	for _, a := range req.Args[1:] {
		cmd.Args = append(cmd.Args, []byte(a))
	}

	frame := s.engine.Execute(cmd)
	writeJSON(w, http.StatusOK, frameToResult(frame))
}

func frameToResult(f resp.Frame) commandResult {
	switch v := f.(type) {
	case resp.SimpleString:
		return commandResult{Type: "string", Value: string(v)}
	case resp.Error:
		msg := v.Message
		if v.Prefix != "" {
			msg = v.Prefix + " " + v.Message
		}
		return commandResult{Type: "error", Value: msg}
	case resp.Integer:
		return commandResult{Type: "integer", Value: int64(v)}
	case resp.BulkString:
		if v.Null {
			return commandResult{Type: "nil"}
		}
		return commandResult{Type: "bulk", Value: string(v.Value)}
	case resp.Array:
		if v.Null {
			return commandResult{Type: "nil"}
		}
		items := make([]commandResult, len(v.Items))
		for i, item := range v.Items {
			items[i] = frameToResult(item)
		}
		return commandResult{Type: "array", Value: items}
	default:
		return commandResult{Type: "error", Value: "unknown frame type"}
	}
}
