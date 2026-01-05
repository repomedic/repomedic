package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"repomedic/internal/rules"
)

func TestEmitSink_JSON(t *testing.T) {
	var buf bytes.Buffer
	s, err := NewEmitSink(&buf, "json")
	if err != nil {
		t.Fatalf("NewEmitSink returned error: %v", err)
	}

	_ = s.Write(rules.Result{Repo: "r", RuleID: "a", Status: rules.StatusPass})
	_ = s.Write(rules.Result{Repo: "r", RuleID: "b", Status: rules.StatusFail})
	if err := s.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	var got []rules.Result
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal json output: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
}

func TestEmitSink_NDJSON(t *testing.T) {
	var buf bytes.Buffer
	s, err := NewEmitSink(&buf, "ndjson")
	if err != nil {
		t.Fatalf("NewEmitSink returned error: %v", err)
	}

	_ = s.Write(rules.Result{Repo: "r", RuleID: "a", Status: rules.StatusPass})
	_ = s.Write(rules.Result{Repo: "r", RuleID: "b", Status: rules.StatusFail})
	if err := s.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ndjson lines, got %d", len(lines))
	}
	for _, line := range lines {
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid json line %q: %v", line, err)
		}
		if e.Type != "rule.result" {
			t.Fatalf("expected event type rule.result, got %q", e.Type)
		}
		if e.Result == nil {
			t.Fatalf("expected event to include result, got nil")
		}
		if e.Repo != "r" {
			t.Fatalf("expected result repo 'r', got %q", e.Repo)
		}
	}
}

func TestEmitSink_InvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewEmitSink(&buf, "text"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestEmitSink_NilWriter(t *testing.T) {
	if _, err := NewEmitSink(nil, "json"); err == nil {
		t.Fatalf("expected error, got nil")
	}
}
