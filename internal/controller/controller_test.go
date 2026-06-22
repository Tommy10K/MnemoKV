package controller

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
)

func TestControllerLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	cfg := config.ClusterConfig{
		FailoverMode: "automatic",
		Controller:   config.ControllerConfig{RaftDir: t.TempDir(), BootstrapNodeID: "node-1"},
		Peers:        []config.PeerConfig{{ID: "node-1", APIAddress: "127.0.0.1:1", ControlAddress: address}},
	}
	c := New(cfg, "node-1")
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctx); err == nil {
		t.Fatal("expected duplicate start to fail")
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := c.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}
