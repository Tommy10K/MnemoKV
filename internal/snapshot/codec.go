package snapshot

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
)

var binaryHeader = []byte("MNEMOKV-SNAPSHOT\x00")

// Encode writes one sealed model in its declared format.
func Encode(w io.Writer, model *Model) error {
	if err := model.Verify(); err != nil {
		return err
	}
	switch model.Format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(model); err != nil {
			return fmt.Errorf("encode JSON snapshot: %w", err)
		}
		return nil
	case FormatBinary:
		if _, err := w.Write(binaryHeader); err != nil {
			return fmt.Errorf("write binary snapshot header: %w", err)
		}
		var payload bytes.Buffer
		if err := gob.NewEncoder(&payload).Encode(model); err != nil {
			return fmt.Errorf("encode binary snapshot: %w", err)
		}
		if err := binary.Write(w, binary.BigEndian, uint64(payload.Len())); err != nil {
			return fmt.Errorf("write binary snapshot length: %w", err)
		}
		if _, err := payload.WriteTo(w); err != nil {
			return fmt.Errorf("write binary snapshot: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported snapshot format %q", model.Format)
	}
}

// Decode reads and verifies one model in the requested format.
func Decode(r io.Reader, format string) (*Model, error) {
	var model Model
	switch format {
	case FormatJSON:
		dec := json.NewDecoder(r)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&model); err != nil {
			return nil, fmt.Errorf("decode JSON snapshot: %w", err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			if err == nil {
				return nil, fmt.Errorf("decode JSON snapshot: trailing value")
			}
			return nil, fmt.Errorf("decode JSON snapshot: %w", err)
		}
	case FormatBinary:
		br := bufio.NewReader(r)
		header := make([]byte, len(binaryHeader))
		if _, err := io.ReadFull(br, header); err != nil {
			return nil, fmt.Errorf("read binary snapshot header: %w", err)
		}
		if !bytes.Equal(header, binaryHeader) {
			return nil, fmt.Errorf("invalid binary snapshot header")
		}
		var length uint64
		if err := binary.Read(br, binary.BigEndian, &length); err != nil {
			return nil, fmt.Errorf("read binary snapshot length: %w", err)
		}
		if length >= uint64(^uint64(0)>>1) {
			return nil, fmt.Errorf("binary snapshot payload is too large")
		}
		payload, err := io.ReadAll(io.LimitReader(br, int64(length)+1))
		if err != nil {
			return nil, fmt.Errorf("read binary snapshot: %w", err)
		}
		if uint64(len(payload)) != length {
			return nil, fmt.Errorf("binary snapshot length is %d bytes, file contains %d", length, len(payload))
		}
		dec := gob.NewDecoder(bytes.NewReader(payload))
		if err := dec.Decode(&model); err != nil {
			return nil, fmt.Errorf("decode binary snapshot: %w", err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return nil, fmt.Errorf("decode binary snapshot: trailing data")
		}
	default:
		return nil, fmt.Errorf("unsupported snapshot format %q", format)
	}
	if model.Format != format {
		return nil, fmt.Errorf("snapshot declares format %q but was read as %q", model.Format, format)
	}
	if err := model.Verify(); err != nil {
		return nil, err
	}
	return &model, nil
}
