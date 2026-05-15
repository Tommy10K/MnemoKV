// Package server owns the TCP entry point for a MnemoKV node. It wires the
// RESP parser/writer to the engine's executor on a per-connection basis and
// manages graceful shutdown.
//
// The server intentionally knows nothing about cluster routing or replication;
// those concerns live in internal/cluster and will be plugged in via the
// Executor abstraction once those phases land.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
)

// Server accepts RESP2 connections and dispatches commands to the engine.
type Server struct {
	cfg     config.NetworkConfig
	engine  *engine.Engine
	metrics metrics.Sink

	listener net.Listener
	wg       sync.WaitGroup

	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closing bool
}

// New builds a Server. The caller is responsible for constructing the engine.
func New(cfg config.NetworkConfig, eng *engine.Engine, sink metrics.Sink) *Server {
	if sink == nil {
		sink = metrics.NewNoop()
	}
	return &Server{
		cfg:     cfg,
		engine:  eng,
		metrics: sink,
		conns:   make(map[net.Conn]struct{}),
	}
}

// Start binds the listener and runs the accept loop until ctx is cancelled or
// Shutdown is called. It returns when the accept loop exits.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.BindAddr, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", addr, err)
	}
	s.listener = ln
	log.Printf("server: listening on %s", addr)

	// Close the listener when the context is cancelled. The accept loop will
	// then exit with a "use of closed connection" error which we map to nil.
	go func() {
		<-ctx.Done()
		_ = s.closeListener()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.isClosing() || errors.Is(err, net.ErrClosed) {
				return nil
			}
			// Transient accept errors should not kill the server; log and retry.
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			log.Printf("server: accept error: %v", err)
			time.Sleep(20 * time.Millisecond)
			continue
		}
		s.trackConn(conn)
		s.wg.Add(1)
		go s.serveConn(conn)
	}
}

// Shutdown stops accepting new connections, closes any open connections, and
// waits for in-flight handlers to finish. It returns when every connection
// goroutine has exited or ctx expires.
func (s *Server) Shutdown(ctx context.Context) error {
	_ = s.closeListener()
	s.closeAllConns()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) closeListener() error {
	s.mu.Lock()
	if s.closing || s.listener == nil {
		s.mu.Unlock()
		return nil
	}
	s.closing = true
	ln := s.listener
	s.mu.Unlock()
	return ln.Close()
}

func (s *Server) isClosing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closing
}

func (s *Server) trackConn(conn net.Conn) {
	s.mu.Lock()
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
}

func (s *Server) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

func (s *Server) closeAllConns() {
	s.mu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
}

// serveConn drives one client connection until it ends or the server stops.
func (s *Server) serveConn(conn net.Conn) {
	defer s.wg.Done()
	defer s.untrackConn(conn)

	h := newConnectionHandler(conn, s.engine, s.metrics, s.cfg)
	h.serve()
}

// canonical RESP error frames returned to clients on framing failures.
var frameProtocolError = resp.NewError("ERR", "Protocol error")
