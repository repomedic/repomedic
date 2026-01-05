package checks

import (
	"context"

	"repomedic/internal/data"
	"repomedic/internal/rules"

	"github.com/google/go-github/v66/github"
)

// SecretScanningDisabledRule detects repositories where GitHub Secret Scanning is available but disabled.
type SecretScanningDisabledRule struct{}

func init() {
	rules.Register(&SecretScanningDisabledRule{})
}

func (r *SecretScanningDisabledRule) ID() string {
	return "secret-scanning-disabled"
}

func (r *SecretScanningDisabledRule) Title() string {
	return "Secret Scanning Available But Disabled"
}

func (r *SecretScanningDisabledRule) Description() string {
	return "Verifies that GitHub Secret Scanning is enabled when it is available for the repository/org.\n\n" +
		"To pass this rule, enable 'Secret scanning' (or 'Secret Protection') in the repository settings.\n" +
		"Location: Settings > Security > Advanced Security.\n" +
		"This rule is gated to only fail if the feature is explicitly reported as available but disabled. " +
		"It does not flag repositories where the feature is unavailable (e.g. missing GHAS license)."
}

func (r *SecretScanningDisabledRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{
		data.DepRepoMetadata,
	}, nil
}

func (r *SecretScanningDisabledRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	// Use metadata from context if available, otherwise fallback to repo argument
	var targetRepo *github.Repository
	if val, ok := dc.Get(data.DepRepoMetadata); ok && val != nil {
		if meta, ok := val.(*github.Repository); ok {
			targetRepo = meta
		}
	}
	if targetRepo == nil {
		targetRepo = repo
	}

	// Gating: Check availability
	if targetRepo.SecurityAndAnalysis == nil {
		return rules.SkippedResult(repo, r.ID(), "Security and analysis settings not available"), nil
	}
	if targetRepo.SecurityAndAnalysis.SecretScanning == nil {
		return rules.SkippedResult(repo, r.ID(), "Secret scanning settings not available"), nil
	}

	status := targetRepo.SecurityAndAnalysis.SecretScanning.GetStatus()

	// If status is enabled, PASS
	if status == "enabled" {
		return rules.PassResultWithMessage(repo, r.ID(), "Secret scanning is enabled"), nil
	}

	// If status is disabled, check if it's allowed or should fail
	if status == "disabled" {
		msg := "Secret scanning is available but disabled"
		evidence := map[string]string{
			"availability": "true",
			"status":       "disabled",
			"repo":         targetRepo.GetFullName(),
		}
		res := rules.FailResult(repo, r.ID(), msg)
		res.Evidence = evidence
		return res, nil
	}

	// Unknown status
	return rules.SkippedResult(repo, r.ID(), "Unknown secret scanning status: "+status), nil
}
