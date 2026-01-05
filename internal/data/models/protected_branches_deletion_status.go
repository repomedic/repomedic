package models

// ProtectedBranchDeletionStatus is a per-scope deletion summary.
//
// A "scope" represents a protection target (typically a wildcard pattern or
// explicit ref pattern) rather than an enumerated branch name. This allows
// wildcard protections to count as one.
//
// DeletionBlocked is true when deletion is blocked for the scope.
//
// Source is a human-readable identifier for where the scope came from
// (e.g. "ruleset" or "classic-branch-protection").
type ProtectedBranchDeletionStatus struct {
	Name            string
	DeletionBlocked bool
	Source          string
	Detail          string
}

// ProtectedBranchesDeletionStatus is a bounded report over protected scopes.
type ProtectedBranchesDeletionStatus struct {
	Branches  []ProtectedBranchDeletionStatus
	Truncated bool
	Limit     int
}
