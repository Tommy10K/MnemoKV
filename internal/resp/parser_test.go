package resp

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

func parseAll(t *testing.T, input string) []*Command {
	t.Helper()
	p := NewParser()
	r := bufio.NewReader(strings.NewReader(input))
	var out []*Command
	for {
		c, err := p.Next(r)
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		out = append(out, c)
	}
}

func TestParseRESPArray(t *testing.T) {
	in := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	cmds := parseAll(t, in)
	if len(cmds) != 1 {
		t.Fatalf("want 1 command, got %d", len(cmds))
	}
	c := cmds[0]
	if c.Name != "SET" {
		t.Fatalf("name=%q", c.Name)
	}
	if len(c.Args) != 2 || string(c.Args[0]) != "foo" || string(c.Args[1]) != "bar" {
		t.Fatalf("args=%q", c.Args)
	}
}

func TestParseInline(t *testing.T) {
	cmds := parseAll(t, "PING\r\n")
	if len(cmds) != 1 || cmds[0].Name != "PING" {
		t.Fatalf("got %+v", cmds)
	}
}

func TestParseEmptyInlineLine(t *testing.T) {
	p := NewParser()
	r := bufio.NewReader(strings.NewReader("\r\n"))
	if _, err := p.Next(r); err != ErrEmptyLine {
		t.Fatalf("expected ErrEmptyLine, got %v", err)
	}
}

func TestParserRejectsMalformed(t *testing.T) {
	p := NewParser()
	r := bufio.NewReader(strings.NewReader("*2\r\n$3\r\nSET\r\nbroken"))
	if _, err := p.Next(r); err == nil {
		t.Fatalf("expected error")
	}
}

func TestParserRejectsEmptyCommandName(t *testing.T) {
	p := NewParser()
	r := bufio.NewReader(strings.NewReader("*1\r\n$0\r\n\r\n"))
	if _, err := p.Next(r); err != ErrEmptyCommand {
		t.Fatalf("expected ErrEmptyCommand, got %v", err)
	}
}

func TestParserRejectsOversizedArray(t *testing.T) {
	p := NewParser()
	r := bufio.NewReader(strings.NewReader("*1048577\r\n"))
	if _, err := p.Next(r); err != ErrProtocol {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}

func TestParserRejectsOversizedAggregateCommand(t *testing.T) {
	p := NewParser()
	p.maxCommandLen = 5
	r := bufio.NewReader(strings.NewReader("*2\r\n$3\r\nSET\r\n$3\r\nkey\r\n"))
	if _, err := p.Next(r); err != ErrProtocol {
		t.Fatalf("expected ErrProtocol, got %v", err)
	}
}

func TestWriterRoundTrip(t *testing.T) {
	cases := map[string]struct {
		frame Frame
		want  string
	}{
		"simple":  {SimpleString("OK"), "+OK\r\n"},
		"error":   {NewError("ERR", "boom"), "-ERR boom\r\n"},
		"int":     {Integer(42), ":42\r\n"},
		"bulk":    {BulkFromString("hi"), "$2\r\nhi\r\n"},
		"nilbulk": {NullBulk, "$-1\r\n"},
		"array":   {Array{Items: []Frame{Integer(1), BulkFromString("x")}}, "*2\r\n:1\r\n$1\r\nx\r\n"},
		"nilarr":  {Array{Null: true}, "*-1\r\n"},
		"empty":   {Array{Items: []Frame{}}, "*0\r\n"},
	}
	for name, tc := range cases {
		var buf bytes.Buffer
		w := NewWriter(&buf)
		if err := w.WriteFrame(tc.frame); err != nil {
			t.Fatalf("%s: write: %v", name, err)
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("%s: flush: %v", name, err)
		}
		if got := buf.String(); got != tc.want {
			t.Fatalf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestWriterSanitizesLineFrames(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteFrame(NewError("ERR\rX", "bad\nreply")); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "-ERR X bad reply\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractPrimaryKey(t *testing.T) {
	c := &Command{Name: "GET", Args: [][]byte{[]byte("k")}}
	if string(ExtractPrimaryKey(c)) != "k" {
		t.Fatalf("expected key")
	}
	c2 := &Command{Name: "PING"}
	if ExtractPrimaryKey(c2) != nil {
		t.Fatalf("expected nil key for PING")
	}
}
