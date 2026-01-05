package data

// DataContext provides fetched GitHub data to rules.
type DataContext interface {
	Get(key DependencyKey) (any, bool)
}

// MapDataContext is a simple read-only map-based implementation of DataContext.
type MapDataContext struct {
	data map[DependencyKey]any
}

func NewMapDataContext(data map[DependencyKey]any) *MapDataContext {
	// A nil map is treated as an empty context.
	// Keeping it nil avoids hidden initialization and ensures the context is read-only.
	return &MapDataContext{data: data}
}

func (c *MapDataContext) Get(key DependencyKey) (any, bool) {
	if c == nil {
		return nil, false
	}
	val, ok := c.data[key]
	return val, ok
}
