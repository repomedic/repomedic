package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"repomedic/internal/rules"
	"strings"
	"sync"
)

type ConsoleSink struct {
	writer          io.Writer
	format          string // "text", "json", "ndjson"
	mu              sync.Mutex
	results         []rules.Result // For JSON array output
	allowedStatuses map[string]bool
}

func NewConsoleSink(w io.Writer, format string, filterStatuses []string) *ConsoleSink {
	if w == nil {
		w = os.Stdout
	}
	if format == "" {
		format = "text"
	}

	s := &ConsoleSink{
		writer: w,
		format: format,
	}

	if len(filterStatuses) > 0 {
		s.allowedStatuses = make(map[string]bool)
		for _, st := range filterStatuses {
			// Normalize to uppercase for case-insensitive comparison
			// The status types are typically "PASS", "FAIL", "ERROR"
			s.allowedStatuses[strings.ToUpper(st)] = true
		}
	}

	return s
}

func (s *ConsoleSink) Write(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(v)
}

func (s *ConsoleSink) writeLocked(v any) error {
	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(s.writer, format, args...)
		return err
	}
	println := func(args ...any) error {
		_, err := fmt.Fprintln(s.writer, args...)
		return err
	}

	// Apply filtering if configured
	if len(s.allowedStatuses) > 0 {
		if r, ok := v.(rules.Result); ok {
			if !s.allowedStatuses[string(r.Status)] {
				return nil
			}
		}
	}

	switch s.format {
	case "json":
		r, ok := v.(rules.Result)
		if !ok {
			// Ignore non-result events in JSON console mode.
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
	case "text":
		r, ok := v.(rules.Result)
		if !ok {
			// Ignore events in text mode.
			return nil
		}
		if err := printf("[%s] %s: %s", r.Status, r.Repo, r.RuleID); err != nil {
			return err
		}
		if r.Message != "" {
			if err := printf(" - %s", r.Message); err != nil {
				return err
			}
		}
		if err := println(); err != nil {
			return err
		}
		return flushIfPossible(s.writer)
	default:
		return fmt.Errorf("unsupported console format: %s", s.format)
	}
}

func (s *ConsoleSink) Close() error {
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
	if s.format != "text" && s.format != "ndjson" {
		return fmt.Errorf("unsupported console format: %s", s.format)
	}
	return nil
}
