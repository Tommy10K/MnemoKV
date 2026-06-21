package cluster

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mnemokv/mnemokv/internal/resp"
)

type RESPProxy struct {
	peers   map[string]string
	timeout time.Duration

	mu    sync.Mutex
	conns map[string]*peerConn
}

type peerConn struct {
	mu sync.Mutex
	c  net.Conn
	r  *bufio.Reader
	w  *bufio.Writer
}

func NewRESPProxy(peers map[string]string, timeout time.Duration) *RESPProxy {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &RESPProxy{
		peers:   peers,
		timeout: timeout,
		conns:   make(map[string]*peerConn),
	}
}

func (p *RESPProxy) Forward(ctx context.Context, nodeID string, cmd *resp.Command) (resp.Frame, error) {
	pc, err := p.getConn(nodeID)
	if err != nil {
		return nil, err
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if dl, ok := ctx.Deadline(); ok {
		_ = pc.c.SetDeadline(dl)
	} else {
		_ = pc.c.SetDeadline(time.Now().Add(p.timeout))
	}

	args := append([]string{cmd.Name}, bytesToStrings(cmd.Args)...)
	if err := writeRequest(pc.w, args); err != nil {
		p.dropConn(nodeID)
		return nil, err
	}
	if err := pc.w.Flush(); err != nil {
		p.dropConn(nodeID)
		return nil, err
	}
	frame, err := readFrame(pc.r)
	if err != nil {
		p.dropConn(nodeID)
		return nil, err
	}
	return frame, nil
}

func (p *RESPProxy) SendReplication(ctx context.Context, nodeID string, rec ReplicationRecord) error {
	pc, err := p.getConn(nodeID)
	if err != nil {
		return err
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if deadline, ok := ctx.Deadline(); ok {
		_ = pc.c.SetDeadline(deadline)
	} else {
		_ = pc.c.SetDeadline(time.Now().Add(p.timeout))
	}

	args := append([]string{
		"REPLICATE", rec.SourceNodeID, strconv.FormatUint(uint64(rec.Slot), 10),
		strconv.FormatUint(rec.Term, 10), strconv.FormatUint(rec.Sequence, 10),
	}, rec.Args...)
	if err := writeRequest(pc.w, args); err != nil {
		p.dropConn(nodeID)
		return err
	}
	if err := pc.w.Flush(); err != nil {
		p.dropConn(nodeID)
		return err
	}
	frame, err := readFrame(pc.r)
	if err != nil {
		p.dropConn(nodeID)
		return err
	}
	if frameErr, ok := frame.(resp.Error); ok {
		if strings.Contains(frameErr.Message, ErrSequenceGap.Error()) {
			return fmt.Errorf("%w: replica %s rejected record", ErrSequenceGap, nodeID)
		}
		if strings.Contains(frameErr.Message, ErrStaleTerm.Error()) {
			return fmt.Errorf("%w: replica %s rejected record", ErrStaleTerm, nodeID)
		}
		return fmt.Errorf("replica %s rejected record: %s %s", nodeID, frameErr.Prefix, frameErr.Message)
	}
	return nil
}

func (p *RESPProxy) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pc := range p.conns {
		_ = pc.c.Close()
	}
	p.conns = make(map[string]*peerConn)
	return nil
}

func (p *RESPProxy) getConn(nodeID string) (*peerConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pc, ok := p.conns[nodeID]; ok {
		return pc, nil
	}
	addr, ok := p.peers[nodeID]
	if !ok {
		return nil, fmt.Errorf("unknown peer %q", nodeID)
	}
	c, err := net.DialTimeout("tcp", addr, p.timeout)
	if err != nil {
		return nil, err
	}
	pc := &peerConn{c: c, r: bufio.NewReader(c), w: bufio.NewWriter(c)}
	p.conns[nodeID] = pc
	return pc, nil
}

func (p *RESPProxy) dropConn(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pc, ok := p.conns[nodeID]; ok {
		_ = pc.c.Close()
		delete(p.conns, nodeID)
	}
}

func writeRequest(w *bufio.Writer, args []string) error {
	if _, err := fmt.Fprintf(w, "*%d\r\n", len(args)); err != nil {
		return err
	}
	for _, a := range args {
		if _, err := fmt.Fprintf(w, "$%d\r\n%s\r\n", len(a), a); err != nil {
			return err
		}
	}
	return nil
}

func readFrame(r *bufio.Reader) (resp.Frame, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, errors.New("short reply")
	}
	prefix, body := line[0], line[1:len(line)-2]
	switch prefix {
	case '+':
		return resp.SimpleString(body), nil
	case '-':
		parts := strings.SplitN(body, " ", 2)
		message := ""
		if len(parts) == 2 {
			message = parts[1]
		}
		return resp.NewError(parts[0], message), nil
	case ':':
		n, err := strconv.ParseInt(body, 10, 64)
		if err != nil {
			return nil, err
		}
		return resp.Integer(n), nil
	case '$':
		n, err := strconv.Atoi(body)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return resp.NullBulk, nil
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return resp.BulkString{Value: buf[:n]}, nil
	case '*':
		n, err := strconv.Atoi(body)
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return resp.Array{Null: true}, nil
		}
		items := make([]resp.Frame, n)
		for i := 0; i < n; i++ {
			f, err := readFrame(r)
			if err != nil {
				return nil, err
			}
			items[i] = f
		}
		return resp.Array{Items: items}, nil
	}
	return nil, fmt.Errorf("unknown prefix %q", prefix)
}

func bytesToStrings(args [][]byte) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = string(a)
	}
	return out
}
