package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"time"

	"github.com/mnemokv/mnemokv/internal/config"
	"github.com/mnemokv/mnemokv/internal/logging"
	"github.com/mnemokv/mnemokv/internal/metrics"
	"github.com/mnemokv/mnemokv/internal/resp"
)

// connectionHandler owns one socket: it reads commands, dispatches them, and
// writes responses. A new handler is created per connection so its parser,
// writer, and read buffer never cross goroutine boundaries.
type connectionHandler struct {
	conn     net.Conn
	reader   *bufio.Reader
	writer   *resp.Writer
	parser   *resp.Parser
	executor CommandExecutor
	sink     metrics.Sink
	cfg      config.NetworkConfig
}

func newConnectionHandler(conn net.Conn, executor CommandExecutor, sink metrics.Sink, cfg config.NetworkConfig) *connectionHandler {
	return &connectionHandler{
		conn:     conn,
		reader:   bufio.NewReaderSize(conn, 32*1024),
		writer:   resp.NewWriterSize(conn, 32*1024),
		parser:   resp.NewParser(),
		executor: executor,
		sink:     sink,
		cfg:      cfg,
	}
}

func (h *connectionHandler) serve() {
	defer func() {
		_ = h.conn.Close()
	}()

	for {
		h.applyReadDeadline()
		cmd, err := h.parser.Next(h.reader)
		if err != nil {
			if errors.Is(err, resp.ErrEmptyLine) {
				continue
			}
			if errors.Is(err, resp.ErrEmptyCommand) {
				_ = h.writeFrame(resp.NewError("ERR", "empty command"))
				continue
			}
			h.handleParseError(err)
			return
		}

		start := time.Now()
		frame := h.executor.Execute(cmd)
		h.sink.ObserveLatency("command_latency", time.Since(start), cmd.Name)

		quit := cmd.Name == "QUIT" && len(cmd.Args) == 0
		resp.Release(cmd)

		if err := h.writeFrame(frame); err != nil {
			// Client likely disconnected; nothing useful we can do.
			return
		}
		if quit {
			return
		}
	}
}

func (h *connectionHandler) writeFrame(f resp.Frame) error {
	h.applyWriteDeadline()
	if err := h.writer.WriteFrame(f); err != nil {
		return err
	}
	return h.writer.Flush()
}

func (h *connectionHandler) applyReadDeadline() {
	if h.cfg.ReadTimeoutMs <= 0 {
		return
	}
	_ = h.conn.SetReadDeadline(time.Now().Add(time.Duration(h.cfg.ReadTimeoutMs) * time.Millisecond))
}

func (h *connectionHandler) applyWriteDeadline() {
	if h.cfg.WriteTimeoutMs <= 0 {
		return
	}
	_ = h.conn.SetWriteDeadline(time.Now().Add(time.Duration(h.cfg.WriteTimeoutMs) * time.Millisecond))
}

func (h *connectionHandler) handleParseError(err error) {
	switch {
	case errors.Is(err, io.EOF):
		// Clean disconnect.
	case errors.Is(err, net.ErrClosed):
		// Listener shut down; drop the connection silently.
	case errors.Is(err, resp.ErrProtocol):
		_ = h.writeFrame(frameProtocolError)
	default:
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			// Idle connection. Close it.
			return
		}
		logging.Warnf("server: read from %s: %v", h.conn.RemoteAddr(), err)
	}
}
