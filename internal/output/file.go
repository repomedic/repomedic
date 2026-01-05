package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"repomedic/internal/rules"
	"strings"
	"sync"
)

type FileSink struct {
	path    string
	format  string
	file    *os.File
	mu      sync.Mutex
	results []rules.Result
}

func NewFileSink(path string, format string) (*FileSink, error) {
	if path == "" {
		return nil, fmt.Errorf("output path required")
	}

	// Infer format if not provided
	if format == "" {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".json":
			format = "json"
		case ".ndjson", ".jsonl":
			format = "ndjson"
		default:
			return nil, fmt.Errorf("cannot infer output format from file extension %q", ext)
		}
	}

	if format != "json" && format != "ndjson" {
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	return &FileSink{
		path:   path,
		format: format,
		file:   f,
	}, nil
}

func (s *FileSink) Write(v any) error {
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
		encoder := json.NewEncoder(s.file)
		switch t := v.(type) {
		case Event:
			return encoder.Encode(t)
		case rules.Result:
			e := eventFromResult(t)
			return encoder.Encode(e)
		default:
			return nil
		}
	}
	return nil
}

func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.format == "json" {
		encoder := json.NewEncoder(s.file)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(s.results)
	}

	if closeErr := s.file.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}
