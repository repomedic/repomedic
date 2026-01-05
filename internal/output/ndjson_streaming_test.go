package output

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmitSink_NDJSON_FlushesPerWrite(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	bw := bufio.NewWriterSize(pw, 64*1024)
	s, err := NewEmitSink(bw, "ndjson")
	if err != nil {
		t.Fatalf("NewEmitSink returned error: %v", err)
	}

	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		r := bufio.NewReader(pr)
		line, err := r.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	if err := s.Write(Event{Type: "run.started"}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	select {
	case line := <-lineCh:
		if !strings.Contains(line, "\"type\":\"run.started\"") {
			t.Fatalf("expected run.started event, got %q", line)
		}
	case err := <-errCh:
		t.Fatalf("read error: %v", err)
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for ndjson line; writer likely not flushing")
	}
}

func TestConsoleSink_NDJSON_FlushesPerWrite(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	bw := bufio.NewWriterSize(pw, 64*1024)
	s := NewConsoleSink(bw, "ndjson")

	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		r := bufio.NewReader(pr)
		line, err := r.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	if err := s.Write(Event{Type: "repo.started", Repo: "org/repo"}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	select {
	case line := <-lineCh:
		if !strings.Contains(line, "\"type\":\"repo.started\"") {
			t.Fatalf("expected repo.started event, got %q", line)
		}
		if !strings.Contains(line, "\"repo\":\"org/repo\"") {
			t.Fatalf("expected repo name in event, got %q", line)
		}
	case err := <-errCh:
		t.Fatalf("read error: %v", err)
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for ndjson line; writer likely not flushing")
	}
}

func TestFileSink_NDJSON_WritesIncrementally(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "out.ndjson")

	s, err := NewFileSink(path, "ndjson")
	if err != nil {
		t.Fatalf("NewFileSink returned error: %v", err)
	}
	defer func() { _ = s.Close() }()

	if err := s.Write(Event{Type: "run.started"}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	b1, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(b1), "\"type\":\"run.started\"") {
		t.Fatalf("expected run.started to be present after first Write, got %q", string(b1))
	}
	if !strings.HasSuffix(string(b1), "\n") {
		t.Fatalf("expected first Write to end with newline, got %q", string(b1))
	}

	if err := s.Write(Event{Type: "run.finished", ExitCode: 0}); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	b2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b2)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ndjson lines after two Writes, got %d: %q", len(lines), string(b2))
	}
}
