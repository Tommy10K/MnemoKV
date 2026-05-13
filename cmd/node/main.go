// Command node is the MnemoKV server binary. It loads a YAML config, builds
// the engine and (placeholder) cluster manager, and runs the RESP listener
// until it receives SIGINT/SIGTERM.
package main

import (
	"context"
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
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/server"
)

func main() {
	configPath := flag.String("config", "configs/standalone.yaml", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("node: starting %s mode=%s", cfg.Node.ID, cfg.Node.Mode)

	sink := metrics.NewInMemory(2048)
	eng := engine.NewWithMetrics(cfg.Engine, sink)
	clusterMgr := cluster.NewManagerWithNode(cfg.Cluster, cfg.Node.ID)
	srv := server.New(cfg.Network, eng, sink)
	apiSrv := api.New(cfg.Observability, cfg.Node, cfg.Cluster, eng, sink, clusterMgr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := clusterMgr.Start(ctx); err != nil {
		log.Fatalf("cluster: start: %v", err)
	}

	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Start(ctx) }()

	apiDone := make(chan error, 1)
	go func() { apiDone <- apiSrv.Start(ctx) }()

	select {
	case <-ctx.Done():
		log.Printf("node: shutdown signal received")
	case err := <-serverDone:
		if err != nil {
			log.Printf("server: exited: %v", err)
		}
	case err := <-apiDone:
		if err != nil {
			log.Printf("api: exited: %v", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server: shutdown: %v", err)
	}
	if err := apiSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("api: shutdown: %v", err)
	}
	if err := clusterMgr.Shutdown(shutdownCtx); err != nil {
		log.Printf("cluster: shutdown: %v", err)
	}
	log.Printf("node: stopped")
}
