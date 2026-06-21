package logging

import (
	"bytes"
	"log"
	"testing"
)

func TestSetLevelFiltersMessages(t *testing.T) {
	var buf bytes.Buffer
	oldOut := log.Writer()
	defer log.SetOutput(oldOut)
	log.SetOutput(&buf)

	if !SetLevel("warn") {
		t.Fatal("expected warn level to be accepted")
	}
	Infof("hidden")
	Warnf("visible")

	if got := buf.String(); got == "" || !bytes.Contains([]byte(got), []byte("visible")) {
		t.Fatalf("expected warn message, got %q", got)
	}
	if bytes.Contains(buf.Bytes(), []byte("hidden")) {
		t.Fatalf("info message should have been filtered: %q", buf.String())
	}

	SetLevel("info")
}

func TestParseLevelRejectsUnknown(t *testing.T) {
	if _, ok := ParseLevel("verbose"); ok {
		t.Fatal("expected unknown level to be rejected")
	}
}
