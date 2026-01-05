package output

import (
	"encoding/json"
	"fmt"
	"io"
	"repomedic/internal/rules"
	"sync"
)

// EmitSink writes additional structured outputs.
//
// Formats:
//   - json: aggregates rule results and writes a single JSON array on Close
//   - ndjson: streams Event values (one JSON object per line)
type EmitSink struct {
	writer  io.Writer
	format  string // "json" | "ndjson"
	mu      sync.Mutex
	results []rules.Result
}

func NewEmitSink(w io.Writer, format string) (*EmitSink, error) {
	if w == nil {
		return nil, fmt.Errorf("emit sink writer must not be nil")
	}
	if format != "json" && format != "ndjson" {
		return nil, fmt.Errorf("unsupported emit format: %s", format)
	}
	return &EmitSink{writer: w, format: format}, nil
}

func (s *EmitSink) Write(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.format {
	case "json":
		r, ok := v.(rules.Result)
		if !ok {
			// Ignore lifecycle events in JSON aggregate mode.
			return nil
		}
		s.results = append(s.results, r)
		return nil
	case "ndjson":
		encoder := json.NewEncoder(s.writer)
		switch t := v.(type) {
		case Event:
			if err := encoder.Encode(t); err != nil {
				return err
			}
			return flushIfPossible(s.writer)
		case rules.Result:
			e := eventFromResult(t)
			if err := encoder.Encode(e); err != nil {
				return err
			}
			return flushIfPossible(s.writer)
		default:
			return nil
		}
	default:
		return fmt.Errorf("unsupported emit format: %s", s.format)
	}
}

func (s *EmitSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.format == "json" {
		encoder := json.NewEncoder(s.writer)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(s.results); err != nil {
			return err
		}
		return flushIfPossible(s.writer)
	}
	return nil
}
