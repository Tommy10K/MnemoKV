package resp

import (
	"bufio"
	"io"
	"strconv"
)

// Writer serializes Frames to an io.Writer using RESP2. Callers are expected
// to wrap their net.Conn in a *bufio.Writer for throughput; Writer.Flush hides
// that detail from the rest of the codebase.
type Writer struct {
	bw *bufio.Writer
}

// NewWriter wraps w in a buffered writer of the default size.
func NewWriter(w io.Writer) *Writer {
	return &Writer{bw: bufio.NewWriter(w)}
}

// NewWriterSize is the same as NewWriter but lets callers tune the buffer.
func NewWriterSize(w io.Writer, size int) *Writer {
	return &Writer{bw: bufio.NewWriterSize(w, size)}
}

// WriteFrame writes a single frame.
func (w *Writer) WriteFrame(f Frame) error {
	switch v := f.(type) {
	case SimpleString:
		return w.writeSimpleString(string(v))
	case Error:
		return w.writeError(v)
	case Integer:
		return w.writeInteger(int64(v))
	case BulkString:
		return w.writeBulk(v)
	case Array:
		return w.writeArray(v)
	default:
		// Should never happen because Frame is a sealed interface in this
		// package, but we fail loudly rather than silently dropping data.
		return ErrProtocol
	}
}

// Flush flushes the underlying buffered writer.
func (w *Writer) Flush() error { return w.bw.Flush() }

func (w *Writer) writeSimpleString(s string) error {
	if err := w.bw.WriteByte('+'); err != nil {
		return err
	}
	if err := writeLineText(w.bw, s); err != nil {
		return err
	}
	_, err := w.bw.WriteString("\r\n")
	return err
}

func (w *Writer) writeError(e Error) error {
	prefix := e.Prefix
	if prefix == "" {
		prefix = "ERR"
	}
	if err := w.bw.WriteByte('-'); err != nil {
		return err
	}
	if err := writeLineText(w.bw, prefix); err != nil {
		return err
	}
	if err := w.bw.WriteByte(' '); err != nil {
		return err
	}
	if err := writeLineText(w.bw, e.Message); err != nil {
		return err
	}
	_, err := w.bw.WriteString("\r\n")
	return err
}

func writeLineText(w *bufio.Writer, value string) error {
	start := 0
	for i := 0; i < len(value); i++ {
		if value[i] != '\r' && value[i] != '\n' {
			continue
		}
		if _, err := w.WriteString(value[start:i]); err != nil {
			return err
		}
		if err := w.WriteByte(' '); err != nil {
			return err
		}
		start = i + 1
	}
	_, err := w.WriteString(value[start:])
	return err
}

func (w *Writer) writeInteger(n int64) error {
	if err := w.bw.WriteByte(':'); err != nil {
		return err
	}
	if _, err := w.bw.WriteString(strconv.FormatInt(n, 10)); err != nil {
		return err
	}
	_, err := w.bw.WriteString("\r\n")
	return err
}

func (w *Writer) writeBulk(b BulkString) error {
	if b.Null {
		_, err := w.bw.WriteString("$-1\r\n")
		return err
	}
	if err := w.bw.WriteByte('$'); err != nil {
		return err
	}
	if _, err := w.bw.WriteString(strconv.Itoa(len(b.Value))); err != nil {
		return err
	}
	if _, err := w.bw.WriteString("\r\n"); err != nil {
		return err
	}
	if _, err := w.bw.Write(b.Value); err != nil {
		return err
	}
	_, err := w.bw.WriteString("\r\n")
	return err
}

func (w *Writer) writeArray(a Array) error {
	if a.Null {
		_, err := w.bw.WriteString("*-1\r\n")
		return err
	}
	if err := w.bw.WriteByte('*'); err != nil {
		return err
	}
	if _, err := w.bw.WriteString(strconv.Itoa(len(a.Items))); err != nil {
		return err
	}
	if _, err := w.bw.WriteString("\r\n"); err != nil {
		return err
	}
	for _, item := range a.Items {
		if err := w.WriteFrame(item); err != nil {
			return err
		}
	}
	return nil
}
