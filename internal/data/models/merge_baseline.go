package models

import "strings"

// MergeMethodMask is a compact representation of allowed merge methods.
//
// It is intended to be deterministic and stable for comparisons and reporting.
// The canonical string order is: merge, squash, rebase.
type MergeMethodMask uint8

const (
	MergeMethodMerge  MergeMethodMask = 1 << iota // merge commits
	MergeMethodSquash                             // squash commits
	MergeMethodRebase                             // rebase commits
)

// Has reports whether the mask contains all bits in other.
func (m MergeMethodMask) Has(other MergeMethodMask) bool {
	return m&other == other
}

// Intersect returns the bitwise intersection of two masks.
func (m MergeMethodMask) Intersect(other MergeMethodMask) MergeMethodMask {
	return m & other
}

// Size returns the number of enabled methods in the mask.
func (m MergeMethodMask) Size() int {
	size := 0
	if m.Has(MergeMethodMerge) {
		size++
	}
	if m.Has(MergeMethodSquash) {
		size++
	}
	if m.Has(MergeMethodRebase) {
		size++
	}
	return size
}

// String returns a deterministic, comma-separated representation of enabled
// methods. If no methods are set, it returns an empty string.
func (m MergeMethodMask) String() string {
	if m == 0 {
		return ""
	}

	parts := make([]string, 0, 3)
	if m.Has(MergeMethodMerge) {
		parts = append(parts, "merge")
	}
	if m.Has(MergeMethodSquash) {
		parts = append(parts, "squash")
	}
	if m.Has(MergeMethodRebase) {
		parts = append(parts, "rebase")
	}

	return strings.Join(parts, ",")
}

// BaselineState describes whether a baseline could be determined.
//
// - set: a baseline is available
// - none: no baseline could be derived
// - conflict: multiple incompatible baselines were observed
type BaselineState string

const (
	BaselineStateSet      BaselineState = "set"
	BaselineStateNone     BaselineState = "none"
	BaselineStateConflict BaselineState = "conflict"
)

// BaselineSource indicates how a baseline was derived.
type BaselineSource string

const (
	BaselineSourceOrganizationRuleset   BaselineSource = "organization_ruleset"
	BaselineSourceConvention            BaselineSource = "convention"
	BaselineSourceRequiredConfiguration BaselineSource = "required_configuration"
)

// MergeBaseline represents a derived baseline for allowed merge methods.
//
// Evidence is an optional list of human-readable notes supporting the baseline.
type MergeBaseline struct {
	State    BaselineState
	Source   BaselineSource
	Allowed  MergeMethodMask
	Evidence []string
}
