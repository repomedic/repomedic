package models

// CodeownersPresence captures whether a CODEOWNERS file exists on the default
// branch in one or both supported locations.
//
// Supported paths:
// - CODEOWNERS
// - .github/CODEOWNERS
//
// Note: This is a scan-time observation; it does not validate CODEOWNERS
// contents.
type CodeownersPresence struct {
	Root   bool
	GitHub bool
}
