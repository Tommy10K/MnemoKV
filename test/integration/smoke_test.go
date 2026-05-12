// Package integration drives the full RESP pipeline against a real listening
// socket. It is intentionally black-box: it only uses the public command
// surface, so a regression in any layer (parser, server, executor, store)
// surfaces here.
package integration

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/engine"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/server"
)

// startServer spins up a server bound to a random local port and returns its
// address. The caller is responsible for ctx cancellation.
func startServer(t *testing.T, ctx context.Context) string {
	t.Helper()

	// Reserve an ephemeral port by listening once and immediately closing.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	addr := probe.Addr().(*net.TCPAddr)
	port := addr.Port
	_ = probe.Close()

	cfg := config.NetworkConfig{
		BindAddr:       "127.0.0.1",
		Port:           port,
		MaxConnections: 64,
		ReadTimeoutMs:  0,
		WriteTimeoutMs: 0,
	}
	eng := engine.New(config.EngineConfig{StripeCount: 16, EvictionPolicy: "noop"})
	srv := server.New(cfg, eng, metrics.NewNoop())

	go func() { _ = srv.Start(ctx) }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	})

	// Wait for the listener to be ready.
	target := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", target, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return target
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server never came up on %s", target)
	return ""
}

// client is a tiny RESP2 client that builds requests as multi-bulk arrays and
// reads replies one frame at a time. It is intentionally minimal because the
// integration test does not need a real client library.
type client struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

func dial(t *testing.T, addr string) *client {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return &client{
		conn:   c,
		reader: bufio.NewReader(c),
		writer: bufio.NewWriter(c),
	}
}

func (c *client) close() { _ = c.conn.Close() }

// send issues a command and returns the textual representation of the reply
// (one of "OK", an integer, a bulk string body, "(nil)", or "ERR ...").
func (c *client) send(t *testing.T, parts ...string) string {
	t.Helper()
	fmt.Fprintf(c.writer, "*%d\r\n", len(parts))
	for _, p := range parts {
		fmt.Fprintf(c.writer, "$%d\r\n%s\r\n", len(p), p)
	}
	if err := c.writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return c.readReply(t)
}

func (c *client) readReply(t *testing.T) string {
	t.Helper()
	line, err := c.reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(line) < 3 {
		t.Fatalf("short reply: %q", line)
	}
	body := strings.TrimRight(line[1:], "\r\n")
	switch line[0] {
	case '+':
		return body
	case '-':
		return "ERR:" + body
	case ':':
		return body
	case '$':
		n, err := strconv.Atoi(body)
		if err != nil {
			t.Fatalf("bad bulk len %q", body)
		}
		if n < 0 {
			return "(nil)"
		}
		buf := make([]byte, n+2)
		if _, err := readFull(c.reader, buf); err != nil {
			t.Fatalf("read bulk: %v", err)
		}
		return string(buf[:n])
	case '*':
		return "ARRAY:" + body
	default:
		t.Fatalf("unknown frame %q", line)
		return ""
	}
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		k, err := r.Read(buf[n:])
		n += k
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func TestSmokeBaselineCommandSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := startServer(t, ctx)

	c := dial(t, addr)
	defer c.close()

	expect := func(name, want string, parts ...string) {
		t.Helper()
		if got := c.send(t, parts...); got != want {
			t.Fatalf("%s: got %q want %q", name, got, want)
		}
	}

	expect("PING", "PONG", "PING")
	expect("SET", "OK", "SET", "foo", "bar")
	expect("GET", "bar", "GET", "foo")
	expect("EXISTS", "1", "EXISTS", "foo")
	expect("DEL", "1", "DEL", "foo")
	expect("EXISTS gone", "0", "EXISTS", "foo")
	expect("GET nil", "(nil)", "GET", "foo")

	expect("INCR 1", "1", "INCR", "ctr")
	expect("INCR 2", "2", "INCR", "ctr")
	expect("INCR 3", "3", "INCR", "ctr")

	expect("SET TTL", "OK", "SET", "tk", "v", "EX", "100")
	expect("EXPIRE", "1", "EXPIRE", "tk", "50")
	got := c.send(t, "TTL", "tk")
	n, err := strconv.Atoi(got)
	if err != nil || n <= 0 || n > 50 {
		t.Fatalf("TTL: %q", got)
	}
	expect("TTL missing", "-2", "TTL", "missing-key")

	expect("FLUSHDB", "OK", "FLUSHDB")
	expect("EXISTS after flush", "0", "EXISTS", "ctr")
}
