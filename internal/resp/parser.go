package resp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
)

// Parser turns a stream of bytes into Commands. It is safe to reuse across
// requests on the same connection but is not safe for concurrent use.
type Parser struct {
	// maxBulkLen caps the size of any single bulk string. It guards against
	// hostile clients claiming to send gigabytes per request.
	maxBulkLen int
}

// NewParser returns a parser with reasonable defaults. The default bulk-string
// cap (32 MiB) matches Redis's behaviour closely enough for the baseline.
func NewParser() *Parser {
	return &Parser{maxBulkLen: 32 * 1024 * 1024}
}

// Next reads a single command from r. It supports two encodings:
//
//   - canonical RESP2 array of bulk strings (what every client library sends)
//   - inline commands (a plain text line, useful for telnet debugging)
//
// On io.EOF the caller should close the connection. ErrProtocol means the
// stream is unusable; the connection must be torn down.
func (p *Parser) Next(r *bufio.Reader) (*Command, error) {
	prefix, err := r.Peek(1)
	if err != nil {
		return nil, err
	}

	if prefix[0] == '*' {
		return p.parseArray(r)
	}
	return p.parseInline(r)
}

func (p *Parser) parseInline(r *bufio.Reader) (*Command, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	parts := splitInline(trimTrailingCRLF(line))
	if len(parts) == 0 {
		return nil, ErrEmptyCommand
	}
	cmd := acquireCommand()
	cmd.Name = upper(parts[0])
	cmd.Args = append(cmd.Args, parts[1:]...)
	return cmd, nil
}

func (p *Parser) parseArray(r *bufio.Reader) (*Command, error) {
	header, err := readLine(r)
	if err != nil {
		return nil, err
	}
	if len(header) < 1 || header[0] != '*' {
		return nil, ErrProtocol
	}
	n, err := strconv.Atoi(string(header[1:]))
	if err != nil || n < 0 {
		return nil, ErrProtocol
	}
	if n == 0 {
		return nil, ErrEmptyCommand
	}

	cmd := acquireCommand()
	for i := 0; i < n; i++ {
		bulk, err := p.readBulk(r)
		if err != nil {
			Release(cmd)
			return nil, err
		}
		if i == 0 {
			cmd.Name = upper(bulk)
		} else {
			cmd.Args = append(cmd.Args, bulk)
		}
	}
	// First bulk is the command name; everything else is an argument. We also
	// accept it as the "key" position when relevant (Args[0]).
	if n == 1 {
		// Commands with no key (PING, COMMAND) end up with Args empty.
		return cmd, nil
	}
	return cmd, nil
}

func (p *Parser) readBulk(r *bufio.Reader) ([]byte, error) {
	header, err := readLine(r)
	if err != nil {
		return nil, err
	}
	if len(header) < 1 || header[0] != '$' {
		return nil, ErrProtocol
	}
	length, err := strconv.Atoi(string(header[1:]))
	if err != nil {
		return nil, ErrProtocol
	}
	if length < 0 {
		// Null bulk is valid in some contexts but not as a command argument.
		return nil, ErrProtocol
	}
	if length > p.maxBulkLen {
		return nil, ErrProtocol
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	// consume the trailing CRLF
	cr, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	lf, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if cr != '\r' || lf != '\n' {
		return nil, ErrProtocol
	}
	return buf, nil
}

// readLine reads one CRLF-terminated line and returns it without the CRLF.
// Lines without a terminator return ErrProtocol so we never silently truncate.
func readLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadSlice('\n')
	if err != nil {
		// ReadSlice returns its buffer on error; if the underlying error is
		// ErrBufferFull the line is too long for our buffer and we treat it as
		// a protocol error.
		if errors.Is(err, bufio.ErrBufferFull) {
			return nil, ErrProtocol
		}
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, ErrProtocol
	}
	// Copy out of the bufio buffer because the next read can overwrite it.
	out := make([]byte, len(line)-2)
	copy(out, line[:len(line)-2])
	return out, nil
}
