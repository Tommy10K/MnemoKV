package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/controlplane"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/logging"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/persistence"
	"github.com/mnemokv/mnemokv/internal/resp"
)

type Snapshotter interface {
	Snapshot() (persistence.Result, error)
}

type CommandExecutor interface {
	Execute(*resp.Command) resp.Frame
}

type Server struct {
	cfg          config.ObservabilityConfig
	node         config.NodeConfig
	cluster      config.ClusterConfig
	controlPlane config.ControlPlaneConfig
	engine       *engine.Engine
	executor     CommandExecutor
	metrics      *metrics.InMemory
	cluMgr       *cluster.Manager
	snapshots    Snapshotter
	fence        *controlplane.FenceStore
	fenceErr     error

	httpSrv *http.Server
}

func New(cfg config.ObservabilityConfig, node config.NodeConfig, clusterCfg config.ClusterConfig, controlPlaneCfg config.ControlPlaneConfig, eng *engine.Engine, sink *metrics.InMemory, cluMgr *cluster.Manager, snapshots Snapshotter) *Server {
	executor := CommandExecutor(eng)
	if cluMgr != nil && cluMgr.Enabled() && cluMgr.Coordinator() != nil {
		executor = cluMgr.Coordinator()
	}
	server := &Server{
		cfg:          cfg,
		node:         node,
		cluster:      clusterCfg,
		controlPlane: controlPlaneCfg,
		engine:       eng,
		executor:     executor,
		metrics:      sink,
		cluMgr:       cluMgr,
		snapshots:    snapshots,
	}
	if clusterCfg.Enabled && clusterCfg.FailoverMode == "automatic" {
		server.fence, server.fenceErr = controlplane.OpenFenceStore(clusterCfg.Controller.RaftDir)
	}
	return server
}

func (s *Server) Start(ctx context.Context) error {
	if s.fenceErr != nil {
		return fmt.Errorf("control-plane fencing: %w", s.fenceErr)
	}
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
		h.Set("Access-Control-Allow-Headers", "Content-Type, X-MnemoKV-Control-Index, X-MnemoKV-Control-Signature")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
