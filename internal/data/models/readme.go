package models

// ReadmePresence captures whether a README exists at repository root on the
// default branch.
//
// This dependency records whether a README was resolved, along with its path.
type ReadmePresence struct {
	Found bool
	Path  string
}
