package output

import (
	"errors"
	"fmt"
)

// Sink defines a destination for scan results.
type Sink interface {
	Write(v any) error
	Close() error
}

// Manager coordinates writing results to multiple sinks.
type Manager struct {
	sinks []Sink
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) AddSink(s Sink) error {
	if m == nil {
		return fmt.Errorf("output manager is nil")
	}
	if s == nil {
		return fmt.Errorf("sink must not be nil")
	}
	m.sinks = append(m.sinks, s)
	return nil
}

func (m *Manager) Write(v any) error {
	if m == nil {
		return fmt.Errorf("output manager is nil")
	}
	var errs []error
	for _, s := range m.sinks {
		if err := s.Write(v); err != nil {
			errs = append(errs, fmt.Errorf("write %T: %w", s, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors writing to sinks: %w", errors.Join(errs...))
	}
	return nil
}

func (m *Manager) Close() error {
	if m == nil {
		return fmt.Errorf("output manager is nil")
	}
	var errs []error
	for _, s := range m.sinks {
		if err := s.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %T: %w", s, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing sinks: %w", errors.Join(errs...))
	}
	return nil
}
