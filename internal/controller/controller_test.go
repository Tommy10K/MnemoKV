package controller

import (
	"context"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
)

func TestControllerLifecycle(t *testing.T) {
	c := New(config.ClusterConfig{FailoverMode: "automatic"}, "node-1")
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
