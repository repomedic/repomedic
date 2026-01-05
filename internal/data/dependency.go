package data

// DependencyKey uniquely identifies a GitHub data dependency.
type DependencyKey string

// DependencyRequest represents a request for a specific dependency with optional parameters.
type DependencyRequest struct {
	Key    DependencyKey
	Params map[string]string
}
