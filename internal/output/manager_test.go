package output

import (
	"errors"
	"strings"
	"testing"
)

type sinkA struct {
	writes   []any
	writeErr error
	closeErr error
}

func (s *sinkA) Write(v any) error {
	s.writes = append(s.writes, v)
	return s.writeErr
}

func (s *sinkA) Close() error {
	return s.closeErr
}

type sinkB struct {
	writes   []any
	writeErr error
	closeErr error
}

func (s *sinkB) Write(v any) error {
	s.writes = append(s.writes, v)
	return s.writeErr
}

func (s *sinkB) Close() error {
	return s.closeErr
}

func TestManager(t *testing.T) {
	t.Run("writes to all sinks", func(t *testing.T) {
		a := &sinkA{}
		b := &sinkB{}

		mgr := NewManager()
		if err := mgr.AddSink(a); err != nil {
			t.Fatalf("AddSink(a) error: %v", err)
		}
		if err := mgr.AddSink(b); err != nil {
			t.Fatalf("AddSink(b) error: %v", err)
		}

		if err := mgr.Write("v1"); err != nil {
			t.Fatalf("Write(v1) error: %v", err)
		}
		if err := mgr.Write("v2"); err != nil {
			t.Fatalf("Write(v2) error: %v", err)
		}
		if err := mgr.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}

		if got := len(a.writes); got != 2 {
			t.Fatalf("sinkA writes: want 2, got %d", got)
		}
		if got := len(b.writes); got != 2 {
			t.Fatalf("sinkB writes: want 2, got %d", got)
		}
	})

	t.Run("AddSink rejects nil", func(t *testing.T) {
		mgr := NewManager()
		if err := mgr.AddSink(nil); err == nil {
			t.Fatalf("AddSink(nil) want error, got nil")
		}
	})

	t.Run("Write aggregates sink errors", func(t *testing.T) {
		a := &sinkA{writeErr: errors.New("boom-a")}
		b := &sinkB{writeErr: errors.New("boom-b")}
		mgr := NewManager()
		if err := mgr.AddSink(a); err != nil {
			t.Fatalf("AddSink(a) error: %v", err)
		}
		if err := mgr.AddSink(b); err != nil {
			t.Fatalf("AddSink(b) error: %v", err)
		}

		err := mgr.Write("v")
		if err == nil {
			t.Fatalf("Write want error, got nil")
		}
		msg := err.Error()
		for _, want := range []string{"errors writing to sinks", "boom-a", "boom-b", "sinkA", "sinkB"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("Write error missing %q; got: %s", want, msg)
			}
		}
	})

	t.Run("Close aggregates sink errors", func(t *testing.T) {
		a := &sinkA{closeErr: errors.New("close-a")}
		b := &sinkB{closeErr: errors.New("close-b")}
		mgr := NewManager()
		if err := mgr.AddSink(a); err != nil {
			t.Fatalf("AddSink(a) error: %v", err)
		}
		if err := mgr.AddSink(b); err != nil {
			t.Fatalf("AddSink(b) error: %v", err)
		}

		err := mgr.Close()
		if err == nil {
			t.Fatalf("Close want error, got nil")
		}
		msg := err.Error()
		for _, want := range []string{"errors closing sinks", "close-a", "close-b", "sinkA", "sinkB"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("Close error missing %q; got: %s", want, msg)
			}
		}
	})
}
