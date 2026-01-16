package output

import (
	"bytes"
	"repomedic/internal/rules"
	"strings"
	"testing"
)

func TestConsoleSink_Filtering(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		filterStatuses []string
		input          rules.Result
		shouldWrite    bool
	}{
		{
			name:           "text - no filter - pass",
			format:         "text",
			filterStatuses: nil,
			input:          rules.Result{Status: rules.StatusPass, Repo: "r", RuleID: "rule"},
			shouldWrite:    true,
		},
		{
			name:           "text - filter FAIL - input PASS",
			format:         "text",
			filterStatuses: []string{"FAIL"},
			input:          rules.Result{Status: rules.StatusPass, Repo: "r", RuleID: "rule"},
			shouldWrite:    false,
		},
		{
			name:           "text - filter FAIL - input FAIL",
			format:         "text",
			filterStatuses: []string{"FAIL"},
			input:          rules.Result{Status: rules.StatusFail, Repo: "r", RuleID: "rule"},
			shouldWrite:    true,
		},
		{
			name:           "text - filter FAIL,ERROR - input ERROR",
			format:         "text",
			filterStatuses: []string{"FAIL", "ERROR"},
			input:          rules.Result{Status: rules.StatusError, Repo: "r", RuleID: "rule"},
			shouldWrite:    true,
		},
		{
			name:           "json - filter FAIL - input PASS",
			format:         "json",
			filterStatuses: []string{"FAIL"},
			input:          rules.Result{Status: rules.StatusPass, Repo: "r", RuleID: "rule"},
			shouldWrite:    false,
		},
		{
			name:           "json - filter FAIL - input FAIL",
			format:         "json",
			filterStatuses: []string{"FAIL"},
			input:          rules.Result{Status: rules.StatusFail, Repo: "r", RuleID: "rule"},
			shouldWrite:    true,
		},
		{
			name:           "text - filter SKIPPED - input SKIPPED",
			format:         "text",
			filterStatuses: []string{"SKIPPED"},
			input:          rules.Result{Status: rules.StatusSkipped, Repo: "r", RuleID: "rule"},
			shouldWrite:    true,
		},
		{
			name:           "text - filter SKIPPED - input PASS",
			format:         "text",
			filterStatuses: []string{"SKIPPED"},
			input:          rules.Result{Status: rules.StatusPass, Repo: "r", RuleID: "rule"},
			shouldWrite:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			sink := NewConsoleSink(&buf, tt.format, tt.filterStatuses)

			err := sink.Write(tt.input)
			if err != nil {
				t.Fatalf("Write error: %v", err)
			}

			output := buf.String()
			// For JSON sink, writes are buffered until Close, specifically for array output?
			// Checking implementation of ConsoleSink.Write:
			// case "json": s.results = append(s.results, r) -> writes nothing to writer yet.
			// cast "text": writes immediately.

			if tt.format == "json" {
				// For JSON, we need to check the internal state or simulate Close?
				// ConsoleSink struct has 'results []rules.Result'.
				// Since we can't easily access 'results' (private), let's check validation via Side Effects if possible,
				// OR we stick to verifying 'text' format which writes immediately,
				// OR we rely on the fact that I modified `writeLocked` which is shared.
				//
				// However, `writeLocked` does the filtering.
				// For "json", `writeLocked` appends to `s.results`.
				// If filtered, it should NOT append.

				// Wait, ConsoleSink.results is private. I cannot check it directly from a test in the same package unless I export it or use reflection,
				// BUT since I am in package `output`, I CAN access private fields!
				if tt.shouldWrite {
					if len(sink.results) != 1 {
						t.Errorf("expected 1 result buffered, got %d", len(sink.results))
					}
				} else {
					if len(sink.results) != 0 {
						t.Errorf("expected 0 results buffered, got %d", len(sink.results))
					}
				}
			} else {
				// Text format
				wroteSomething := len(output) > 0
				if tt.shouldWrite && !wroteSomething {
					t.Errorf("expected output, got none")
				}
				if !tt.shouldWrite && wroteSomething {
					t.Errorf("expected no output, got: %q", output)
				}
			}
		})
	}
}

func TestConsoleSink_Filtering_CaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	// Filter is "fail", input is "FAIL"
	sink := NewConsoleSink(&buf, "text", []string{"fail"})

	input := rules.Result{Status: rules.StatusFail, Repo: "r", RuleID: "rule"}
	if err := sink.Write(input); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected output for case-insensitive match, got none")
	}
}

func TestConsoleSink_Filtering_NDJSON(t *testing.T) {
	// NDJSON writes immediately
	var buf bytes.Buffer
	sink := NewConsoleSink(&buf, "ndjson", []string{"FAIL"})

	// PASS should be ignored
	pass := rules.Result{Status: rules.StatusPass, Repo: "r", RuleID: "rule"}
	if err := sink.Write(pass); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if buf.Len() > 0 {
		t.Errorf("expected no output for PASS, got: %s", buf.String())
	}

	// FAIL should be written
	fail := rules.Result{Status: rules.StatusFail, Repo: "r", RuleID: "rule"}
	if err := sink.Write(fail); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if !strings.Contains(buf.String(), `"status":"FAIL"`) {
		t.Errorf("expected output for FAIL, got: %s", buf.String())
	}
}
