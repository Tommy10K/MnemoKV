package snapshot

import (
	"bytes"
	"testing"
	"time"
)

func TestCodecsRoundTripSharedModel(t *testing.T) {
	for _, format := range []string{FormatJSON, FormatBinary} {
		t.Run(format, func(t *testing.T) {
			model := &Model{
				Format: format, FormatVersion: FormatVersion, NodeID: "node-1", ClusterID: "cluster-1",
				CreatedAt: time.Unix(123, 456).UTC(), SlotCount: 2, MetadataVersion: 7,
				Slots:   []Slot{{Number: 1, Role: "replica", Term: 3, LastAppliedSequence: 9}},
				Entries: []Entry{{Key: "k", ValueType: "string", Value: []byte{0, 1, 2}, ApproxSize: 68}},
			}
			if err := model.Seal(); err != nil {
				t.Fatal(err)
			}
			var encoded bytes.Buffer
			if err := Encode(&encoded, model); err != nil {
				t.Fatal(err)
			}
			decoded, err := Decode(bytes.NewReader(encoded.Bytes()), format)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Checksum != model.Checksum || !bytes.Equal(decoded.Entries[0].Value, model.Entries[0].Value) {
				t.Fatalf("round trip mismatch: %#v", decoded)
			}
		})
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	model := &Model{Format: FormatJSON, FormatVersion: FormatVersion, NodeID: "n", CreatedAt: time.Now().UTC(), Entries: []Entry{{Key: "k", ValueType: "string", Value: []byte("v")}}}
	if err := model.Seal(); err != nil {
		t.Fatal(err)
	}
	model.Entries[0].Value[0] = 'x'
	if err := model.Verify(); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestBinaryCodecRejectsTrailingData(t *testing.T) {
	model := &Model{Format: FormatBinary, FormatVersion: FormatVersion, NodeID: "n", CreatedAt: time.Now().UTC(), Entries: []Entry{}}
	if err := model.Seal(); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, model); err != nil {
		t.Fatal(err)
	}
	encoded.WriteByte(0)
	if _, err := Decode(bytes.NewReader(encoded.Bytes()), FormatBinary); err == nil {
		t.Fatal("expected trailing binary data to fail")
	}
}
