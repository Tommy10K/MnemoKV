// Command node is the MnemoKV server binary. It loads a YAML config, builds
// the engine and (placeholder) cluster manager, and runs the RESP listener
// until it receives SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mnemokv/mnemokv/internal/api"
	"github.com/mnemokv/mnemokv/internal/cluster"
	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/logging"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/persistence"
	"github.com/mnemokv/mnemokv/internal/server"
)

func main() {
	configPath := flag.String("config", "configs/standalone.yaml", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	logging.SetLevel(cfg.Observability.LogLevel)
	logging.Infof("node: starting %s mode=%s", cfg.Node.ID, cfg.Node.Mode)

	sink := metrics.NewInMemory(2048)
	eng := engine.NewWithMetrics(cfg.Engine, sink)
	clusterMgr := cluster.NewManagerWithNode(cfg.Cluster, cfg.Node.ID)
	clusterMgr.AttachEngine(eng)
	snapshotMgr := persistence.New(cfg.Persistence, cfg.Node.ID, eng, clusterMgr.SnapshotMetadata)
	snapshotMgr.SetMetadataRestorer(clusterMgr.RestoreMetadata)
	if cfg.Persistence.Enabled && cfg.Persistence.LoadOnStart {
		restored, restoreErr := snapshotMgr.RestoreLatest()
		switch {
		case restoreErr == nil:
			logging.Infof("persistence: restored %d entries from %s", restored.RestoredEntries, restored.Path)
		case errors.Is(restoreErr, persistence.ErrNoSnapshot):
			logging.Infof("persistence: no snapshot found; starting with an empty dataset")
		default:
			log.Fatalf("persistence: restore: %v", restoreErr)
		}
	}
	var commandExecutor server.CommandExecutor = eng
	if cfg.Cluster.Enabled {
		commandExecutor = clusterMgr.Coordinator()
	}
	srv := server.New(cfg.Network, commandExecutor, sink)
	apiSrv := api.New(cfg.Observability, cfg.Node, cfg.Cluster, eng, sink, clusterMgr, snapshotMgr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := clusterMgr.Start(ctx); err != nil {
		log.Fatalf("cluster: start: %v", err)
	}
	snapshotMgr.Start(ctx)

	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Start(ctx) }()

	apiDone := make(chan error, 1)
	go func() { apiDone <- apiSrv.Start(ctx) }()

	select {
	case <-ctx.Done():
		logging.Infof("node: shutdown signal received")
	case err := <-serverDone:
		if err != nil {
			logging.Errorf("server: exited: %v", err)
		}
	case err := <-apiDone:
		if err != nil {
			logging.Errorf("api: exited: %v", err)
		}
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logging.Warnf("server: shutdown: %v", err)
	}
	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		logging.Warnf("api: shutdown: %v", err)
	}
	if err := clusterMgr.Shutdown(shutdownCtx); err != nil {
		logging.Warnf("cluster: shutdown: %v", err)
	}
	snapshotMgr.Wait()
	logging.Infof("node: stopped")
}
