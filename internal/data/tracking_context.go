package data

import "sort"

// TrackingDataContext wraps another DataContext and records every dependency key
// that callers attempt to read via Get().
//
// This is primarily used by the engine to enforce the contract that rules must
// declare all dependencies up front via Rule.Dependencies().
type TrackingDataContext struct {
	inner    DataContext
	accessed map[DependencyKey]struct{}
}

func NewTrackingDataContext(inner DataContext) *TrackingDataContext {
	return &TrackingDataContext{
		inner:    inner,
		accessed: make(map[DependencyKey]struct{}),
	}
}

func (c *TrackingDataContext) Get(key DependencyKey) (any, bool) {
	if c == nil {
		return nil, false
	}
	c.accessed[key] = struct{}{}
	if c.inner == nil {
		return nil, false
	}
	return c.inner.Get(key)
}

func (c *TrackingDataContext) AccessedKeys() []DependencyKey {
	if c == nil {
		return nil
	}
	keys := make([]DependencyKey, 0, len(c.accessed))
	for k := range c.accessed {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
