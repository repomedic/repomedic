package data

// FetchScope declares the caching/singleflight scope for a dependency fetch.
//
// RepoMedic fetchers are typically repo-scoped, but some dependencies can be
// org-scoped so they are fetched once per org and reused across repositories.
type FetchScope string

const (
	ScopeRepo FetchScope = "repo"
	ScopeOrg  FetchScope = "org"
)

const (
	// DepRepoMetadata represents the repository metadata (usually fetched during discovery, but can be a dependency).
	DepRepoMetadata DependencyKey = "repo.metadata"

	// DepRepoDefaultBranchClassicProtection represents the classic branch protection object
	// for the default branch.
	DepRepoDefaultBranchClassicProtection DependencyKey = "repo.default_branch_protection"

	// DepRepoDefaultBranchEffectiveRules represents the effective rules that apply to the repository's
	// default branch.
	//
	// This includes rulesets, including inherited org rulesets.
	DepRepoDefaultBranchEffectiveRules DependencyKey = "repo.default_branch_rules"

	// DepRepoDefaultBranchCodeowners represents the presence of a CODEOWNERS file on
	// the repository's default branch.
	//
	// This dependency captures presence in the supported locations:
	// - CODEOWNERS
	// - .github/CODEOWNERS
	DepRepoDefaultBranchCodeowners DependencyKey = "repo.default_branch_codeowners"

	// DepRepoDefaultBranchReadme represents the presence of a repository README on
	// the default branch.
	//
	// This dependency records whether a README was resolved, along with the path.
	DepRepoDefaultBranchReadme DependencyKey = "repo.default_branch_readme"

	// DepRepoProtectedBranchesDeletionStatus represents a bounded report over the
	// repository's protected branches (classic branch protection and/or rulesets),
	// including whether deletion is blocked for each protected branch.
	DepRepoProtectedBranchesDeletionStatus DependencyKey = "repo.protected_branches_deletion_status"

	// DepRepoClassicBranchProtections represents a bounded list of classic branch
	// protection rules defined on the repository.
	DepRepoClassicBranchProtections DependencyKey = "repo.classic_branch_protections"

	// DepRepoAllRulesets represents all rulesets (including inherited org rulesets)
	// that apply to the repository. This includes their enforcement status (active,
	// evaluate, disabled).
	DepRepoAllRulesets DependencyKey = "repo.all_rulesets"

	// DepOrgMergeBaseline represents the organization-level merge-method baseline.
	//
	// Value type: *models.MergeBaseline
	DepOrgMergeBaseline DependencyKey = "org.merge_baseline"

	// DepReposScanned represents the repositories discovered for the current scan.
	//
	// Value type: []*github.Repository
	DepReposScanned DependencyKey = "org.repos_scanned"

	// DepReposMergeConvention represents a convention baseline derived from a sample
	// of scanned repositories.
	//
	// Value type: *models.MergeBaseline
	DepReposMergeConvention DependencyKey = "org.merge_convention"

	// DepMergeBaseline represents the final selected merge-method baseline.
	//
	// Value type: *models.MergeBaseline
	DepMergeBaseline DependencyKey = "org.merge_baseline_selected"

	// DepRepoEffectiveMergeMethods represents the effective allowed merge methods for
	// the repository's default branch, considering repo settings and applicable rulesets.
	//
	// Value type: models.MergeMethodMask
	DepRepoEffectiveMergeMethods DependencyKey = "repo.effective_merge_methods"
)

// Priority returns the fetch priority for a dependency key (lower is higher priority).
func Priority(key DependencyKey) int {
	switch key {
	case DepRepoMetadata:
		return 0 // Highest priority (P0)
	case DepRepoDefaultBranchClassicProtection, DepRepoDefaultBranchEffectiveRules, DepRepoProtectedBranchesDeletionStatus, DepRepoAllRulesets:
		return 1 // Governance config (P1)
	default:
		return 2 // Everything else (P2)
	}
}
