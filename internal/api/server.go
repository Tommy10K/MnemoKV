package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/logging"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/persistence"
)

type Snapshotter interface {
	Snapshot() (persistence.Result, error)
}

type Server struct {
	cfg       config.ObservabilityConfig
	node      config.NodeConfig
	cluster   config.ClusterConfig
	engine    *engine.Engine
	metrics   *metrics.InMemory
	cluMgr    *cluster.Manager
	snapshots Snapshotter

	httpSrv *http.Server
}

func New(cfg config.ObservabilityConfig, node config.NodeConfig, clusterCfg config.ClusterConfig, eng *engine.Engine, sink *metrics.InMemory, cluMgr *cluster.Manager, snapshots Snapshotter) *Server {
	return &Server{
		cfg:       cfg,
		node:      node,
		cluster:   clusterCfg,
		engine:    eng,
		metrics:   sink,
		cluMgr:    cluMgr,
		snapshots: snapshots,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	addr := fmt.Sprintf("%s:%d", s.cfg.APIBindAddr, s.cfg.APIPort)
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	logging.Infof("api: listening on %s", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// withCORS allows the local frontend dev server (and any other browser client)
// to talk to the API. The browser sends an OPTIONS preflight before non-GET
// requests with a JSON body; we answer it with the same headers and 204.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
