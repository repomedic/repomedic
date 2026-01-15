package providers_test

import (
	"context"
	"testing"

	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"
	_ "repomedic/internal/fetcher/providers"

	"github.com/google/go-github/v81/github"
)

func TestMergeBaselineFetcher_Registration(t *testing.T) {
	df, ok := fetcher.ResolveDataFetcher(data.DepMergeBaseline)
	if !ok {
		t.Fatal("expected data fetcher registered for DepMergeBaseline")
	}
	if df.Key() != data.DepMergeBaseline {
		t.Errorf("Key() = %q, want %q", df.Key(), data.DepMergeBaseline)
	}
	if df.Scope() != data.ScopeOrg {
		t.Errorf("Scope() = %q, want %q", df.Scope(), data.ScopeOrg)
	}
}

func TestOrgMergeBaselineFetcher_Registration(t *testing.T) {
	df, ok := fetcher.ResolveDataFetcher(data.DepOrgMergeBaseline)
	if !ok {
		t.Fatal("expected data fetcher registered for DepOrgMergeBaseline")
	}
	if df.Key() != data.DepOrgMergeBaseline {
		t.Errorf("Key() = %q, want %q", df.Key(), data.DepOrgMergeBaseline)
	}
	if df.Scope() != data.ScopeOrg {
		t.Errorf("Scope() = %q, want %q", df.Scope(), data.ScopeOrg)
	}
}

func TestReposMergeConventionFetcher_Registration(t *testing.T) {
	df, ok := fetcher.ResolveDataFetcher(data.DepReposMergeConvention)
	if !ok {
		t.Fatal("expected data fetcher registered for DepReposMergeConvention")
	}
	if df.Key() != data.DepReposMergeConvention {
		t.Errorf("Key() = %q, want %q", df.Key(), data.DepReposMergeConvention)
	}
	if df.Scope() != data.ScopeOrg {
		t.Errorf("Scope() = %q, want %q", df.Scope(), data.ScopeOrg)
	}
}

func TestRepoEffectiveMergeMethodsFetcher_Registration(t *testing.T) {
	df, ok := fetcher.ResolveDataFetcher(data.DepRepoEffectiveMergeMethods)
	if !ok {
		t.Fatal("expected data fetcher registered for DepRepoEffectiveMergeMethods")
	}
	if df.Key() != data.DepRepoEffectiveMergeMethods {
		t.Errorf("Key() = %q, want %q", df.Key(), data.DepRepoEffectiveMergeMethods)
	}
	if df.Scope() != data.ScopeRepo {
		t.Errorf("Scope() = %q, want %q", df.Scope(), data.ScopeRepo)
	}
}

func TestMergeBaselineModels(t *testing.T) {
	// Test that the models package is correctly imported and usable.
	baseline := &models.MergeBaseline{
		State:    models.BaselineStateSet,
		Source:   models.BaselineSourceOrganizationRuleset,
		Allowed:  models.MergeMethodSquash,
		Evidence: []string{"test evidence"},
	}

	if baseline.State != models.BaselineStateSet {
		t.Errorf("State = %q, want %q", baseline.State, models.BaselineStateSet)
	}

	if baseline.Source != models.BaselineSourceOrganizationRuleset {
		t.Errorf("Source = %q, want %q", baseline.Source, models.BaselineSourceOrganizationRuleset)
	}

	if baseline.Allowed != models.MergeMethodSquash {
		t.Errorf("Allowed = %v, want %v", baseline.Allowed, models.MergeMethodSquash)
	}
}

func TestReposMergeConventionFetcher_EmptyRepos(t *testing.T) {
	f := newTestFetcher(t)

	// Inject empty scanned repos.
	f.SetScannedRepos([]*github.Repository{})

	repo := &github.Repository{
		Owner: &github.User{Login: github.Ptr("org")},
		Name:  github.Ptr("repo1"),
	}

	result, err := f.Fetch(context.Background(), repo, data.DepReposMergeConvention, nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	baseline, ok := result.(*models.MergeBaseline)
	if !ok {
		t.Fatalf("Fetch returned type %T, want *models.MergeBaseline", result)
	}

	if baseline.State != models.BaselineStateNone {
		t.Errorf("State = %q, want %q", baseline.State, models.BaselineStateNone)
	}
}

func TestReposMergeConventionFetcher_WithRepos(t *testing.T) {
	f := newTestFetcher(t)

	// Inject scanned repos with merge method settings.
	scannedRepos := []*github.Repository{
		{
			FullName:         github.Ptr("org/repo1"),
			Owner:            &github.User{Login: github.Ptr("org")},
			Name:             github.Ptr("repo1"),
			DefaultBranch:    github.Ptr("main"),
			AllowMergeCommit: github.Ptr(false),
			AllowSquashMerge: github.Ptr(true),
			AllowRebaseMerge: github.Ptr(false),
		},
		{
			FullName:         github.Ptr("org/repo2"),
			Owner:            &github.User{Login: github.Ptr("org")},
			Name:             github.Ptr("repo2"),
			DefaultBranch:    github.Ptr("main"),
			AllowMergeCommit: github.Ptr(false),
			AllowSquashMerge: github.Ptr(true),
			AllowRebaseMerge: github.Ptr(false),
		},
		{
			FullName:         github.Ptr("org/repo3"),
			Owner:            &github.User{Login: github.Ptr("org")},
			Name:             github.Ptr("repo3"),
			DefaultBranch:    github.Ptr("main"),
			AllowMergeCommit: github.Ptr(true),
			AllowSquashMerge: github.Ptr(true),
			AllowRebaseMerge: github.Ptr(true),
		},
	}
	f.SetScannedRepos(scannedRepos)

	repo := &github.Repository{
		Owner: &github.User{Login: github.Ptr("org")},
		Name:  github.Ptr("repo1"),
	}

	result, err := f.Fetch(context.Background(), repo, data.DepReposMergeConvention, nil)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	baseline, ok := result.(*models.MergeBaseline)
	if !ok {
		t.Fatalf("Fetch returned type %T, want *models.MergeBaseline", result)
	}

	if baseline.State != models.BaselineStateSet {
		t.Errorf("State = %q, want %q", baseline.State, models.BaselineStateSet)
	}

	// squash-only (2 repos) should win over all methods (1 repo).
	if baseline.Allowed != models.MergeMethodSquash {
		t.Errorf("Allowed = %q, want %q", baseline.Allowed.String(), models.MergeMethodSquash.String())
	}
}
