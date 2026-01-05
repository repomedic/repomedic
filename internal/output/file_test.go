package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"repomedic/internal/rules"
	"strings"
	"testing"
)

func newTempFilePath(t *testing.T, pattern string) string {
	t.Helper()

	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	return path
}

func TestNewFileSink_InferFormat_FromExtension(t *testing.T) {
	path := newTempFilePath(t, "sink_*.json")
	defer os.Remove(path)

	s, err := NewFileSink(path, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_ = s.Close()
}

func TestNewFileSink_InferFormat_NDJSON_FromExtension(t *testing.T) {
	path := newTempFilePath(t, "sink_*.ndjson")
	defer os.Remove(path)

	s, err := NewFileSink(path, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_ = s.Close()
}

func TestNewFileSink_UnknownExtension_Errors_WhenFormatOmitted(t *testing.T) {
	path := newTempFilePath(t, "sink_*.unknown")
	defer os.Remove(path)

	_, err := NewFileSink(path, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot infer output format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewFileSink_UnsupportedFormat_Errors(t *testing.T) {
	path := newTempFilePath(t, "sink_*.json")
	defer os.Remove(path)

	_, err := NewFileSink(path, "xml")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported output format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFileSink_JSON_AggregatesResults_AndIgnoresEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")

	s, err := NewFileSink(path, "json")
	if err != nil {
		t.Fatalf("NewFileSink failed: %v", err)
	}

	if err := s.Write(Event{Type: "run.started"}); err != nil {
		t.Fatalf("Write event failed: %v", err)
	}
	if err := s.Write(rules.Result{RuleID: "r1", Repo: "o/r", Status: rules.StatusPass}); err != nil {
		t.Fatalf("Write result failed: %v", err)
	}
	if err := s.Write(rules.Result{RuleID: "r2", Repo: "o/r", Status: rules.StatusFail, Message: "nope"}); err != nil {
		t.Fatalf("Write result failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var got []rules.Result
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v\nbody=%s", err, string(b))
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].RuleID != "r1" || got[1].RuleID != "r2" {
		t.Fatalf("unexpected results order/content: %#v", got)
	}
}

func TestFileSink_NDJSON_StreamsEventsAndResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.ndjson")

	s, err := NewFileSink(path, "")
	if err != nil {
		t.Fatalf("NewFileSink failed: %v", err)
	}

	if err := s.Write(Event{Type: "run.started"}); err != nil {
		t.Fatalf("Write event failed: %v", err)
	}
	if err := s.Write(rules.Result{RuleID: "r1", Repo: "o/r", Status: rules.StatusPass}); err != nil {
		t.Fatalf("Write result failed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 ndjson lines, got %d\nbody=%s", len(lines), string(b))
	}

	var e1 Event
	if err := json.Unmarshal([]byte(lines[0]), &e1); err != nil {
		t.Fatalf("Unmarshal line 1 failed: %v", err)
	}
	if e1.Type != "run.started" {
		t.Fatalf("unexpected event type: %q", e1.Type)
	}

	var e2 Event
	if err := json.Unmarshal([]byte(lines[1]), &e2); err != nil {
		t.Fatalf("Unmarshal line 2 failed: %v", err)
	}
	if e2.Type != "rule.result" || e2.Result == nil {
		t.Fatalf("unexpected rule.result event: %#v", e2)
	}
	if e2.Result.RuleID != "r1" {
		t.Fatalf("unexpected result payload: %#v", e2.Result)
	}
}
