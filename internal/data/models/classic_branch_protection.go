package models

// ClassicBranchProtection represents a single classic branch protection rule.
type ClassicBranchProtection struct {
	Pattern         string
	IsAdminEnforced bool
}

// ClassicBranchProtections represents a bounded list of classic branch protection rules.
type ClassicBranchProtections struct {
	Protections []ClassicBranchProtection
	Truncated   bool
	Limit       int
}
