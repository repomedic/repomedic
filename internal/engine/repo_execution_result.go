package engine

import "repomedic/internal/data"

// RepoExecutionResult represents the outcome of executing (fetching) all planned
// dependencies for a single repository.
//
// It is emitted by the scheduler and consumed by the engine during streaming
// scan execution.
type RepoExecutionResult struct {
	RepoID  int64
	Data    data.DataContext
	DepErrs map[data.DependencyKey]error
}
