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
	"github.com/mnemokv/mnemokv/internal/metrics"
)

type Server struct {
	cfg     config.ObservabilityConfig
	node    config.NodeConfig
	cluster config.ClusterConfig
	engine  *engine.Engine
	metrics *metrics.InMemory
	cluMgr  *cluster.Manager

	httpSrv *http.Server
}

func New(cfg config.ObservabilityConfig, node config.NodeConfig, clusterCfg config.ClusterConfig, eng *engine.Engine, sink *metrics.InMemory, cluMgr *cluster.Manager) *Server {
	return &Server{
		cfg:     cfg,
		node:    node,
		cluster: clusterCfg,
		engine:  eng,
		metrics: sink,
		cluMgr:  cluMgr,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	addr := fmt.Sprintf("%s:%d", s.cfg.APIBindAddr, s.cfg.APIPort)
	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

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
