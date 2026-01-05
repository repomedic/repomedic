package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"
	gh "repomedic/internal/github"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

func newTestScheduler(t *testing.T, mux *http.ServeMux, concurrency int) *Scheduler {
	t.Helper()

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := github.NewClient(nil)
	u, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	budget := fetcher.NewRequestBudget()
	f := fetcher.NewFetcher(ghClient, budget)

	scheduler, err := NewScheduler(f, concurrency)
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}
	return scheduler
}

func TestScheduler_Execute_Stream_SingleRepoSuccess(t *testing.T) {
	// Mock GitHub API
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"owner/repo", "default_branch":"main"}`)
	})
	// Mock metadata fetch (which calls GET /repos/owner/repo)
	// Mock branch protection (GET /repos/owner/repo/branches/main/protection)
	mux.HandleFunc("/repos/owner/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"url": "protection_url"}`)
	})

	scheduler := newTestScheduler(t, mux, 2)

	// Create Plan
	repo := RepositoryRef{
		ID:   1,
		Name: "repo",
		Repo: &github.Repository{
			ID:       github.Int64(1),
			Name:     github.String("repo"),
			Owner:    &github.User{Login: github.String("owner")},
			FullName: github.String("owner/repo"),
		},
	}

	plan := NewScanPlan()
	// Manually add repo plan with dependencies
	rp := &RepoPlan{
		Repo: repo,
		Dependencies: map[data.DependencyKey]data.DependencyRequest{
			data.DepRepoMetadata:                       {Key: data.DepRepoMetadata},
			data.DepRepoDefaultBranchClassicProtection: {Key: data.DepRepoDefaultBranchClassicProtection},
		},
	}
	plan.RepoPlans[1] = rp

	// Execute (streaming)
	ctx := context.Background()
	resCh, errCh := scheduler.Execute(ctx, plan)

	var res RepoExecutionResult
	count := 0
	for r := range resCh {
		res = r
		count++
	}
	if count != 1 {
		t.Fatalf("Expected exactly 1 streamed result, got %d", count)
	}
	if res.RepoID != 1 {
		t.Fatalf("Expected result for repo 1, got %d", res.RepoID)
	}
	if _, ok := res.Data.Get(data.DepRepoMetadata); !ok {
		t.Error("Missing metadata result")
	}
	if _, ok := res.Data.Get(data.DepRepoDefaultBranchClassicProtection); !ok {
		t.Error("Missing branch protection result")
	}
	if got := len(res.DepErrs); got != 0 {
		t.Fatalf("Expected no dependency errors, got %d (%v)", got, res.DepErrs)
	}
	for err := range errCh {
		if err != nil {
			t.Fatalf("Expected no fatal scheduler error, got %v", err)
		}
	}
}

func TestScheduler_Execute_Stream_SurfacesDependencyErrors(t *testing.T) {
	// Mock GitHub API
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"owner/repo", "default_branch":"main"}`)
	})
	// Force branch protection to fail
	mux.HandleFunc("/repos/owner/repo/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"boom"}`)
	})

	scheduler := newTestScheduler(t, mux, 2)

	repo := RepositoryRef{
		ID:   1,
		Name: "repo",
		Repo: &github.Repository{
			ID:       github.Int64(1),
			Name:     github.String("repo"),
			Owner:    &github.User{Login: github.String("owner")},
			FullName: github.String("owner/repo"),
		},
	}

	plan := NewScanPlan()
	rp := &RepoPlan{
		Repo: repo,
		Dependencies: map[data.DependencyKey]data.DependencyRequest{
			data.DepRepoMetadata:                       {Key: data.DepRepoMetadata},
			data.DepRepoDefaultBranchClassicProtection: {Key: data.DepRepoDefaultBranchClassicProtection},
		},
	}
	plan.RepoPlans[1] = rp

	ctx := context.Background()
	resCh, errCh := scheduler.Execute(ctx, plan)

	var res RepoExecutionResult
	count := 0
	for r := range resCh {
		res = r
		count++
	}
	if count != 1 {
		t.Fatalf("Expected exactly 1 streamed result, got %d", count)
	}

	if _, ok := res.Data.Get(data.DepRepoMetadata); !ok {
		t.Error("Missing metadata result")
	}
	if _, ok := res.Data.Get(data.DepRepoDefaultBranchClassicProtection); ok {
		t.Error("Expected missing branch protection result when fetch fails")
	}
	if res.DepErrs[data.DepRepoDefaultBranchClassicProtection] == nil {
		t.Fatalf("Expected error recorded for %s", data.DepRepoDefaultBranchClassicProtection)
	}
	for err := range errCh {
		if err != nil {
			t.Fatalf("Expected no fatal scheduler error, got %v", err)
		}
	}
}

func TestScheduler_Execute_Stream_NReposExactlyNResults_AndChannelsClose(t *testing.T) {
	mux := http.NewServeMux()

	// Repo metadata
	mux.HandleFunc("/repos/owner/repo1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo1", "full_name":"owner/repo1", "default_branch":"main"}`)
	})
	mux.HandleFunc("/repos/owner/repo2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":2, "name":"repo2", "full_name":"owner/repo2", "default_branch":"main"}`)
	})
	mux.HandleFunc("/repos/owner/repo3", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":3, "name":"repo3", "full_name":"owner/repo3", "default_branch":"main"}`)
	})

	// Branch protection
	mux.HandleFunc("/repos/owner/repo1/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"url": "protection_url"}`)
	})
	mux.HandleFunc("/repos/owner/repo2/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"url": "protection_url"}`)
	})
	mux.HandleFunc("/repos/owner/repo3/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"url": "protection_url"}`)
	})

	scheduler := newTestScheduler(t, mux, 2)

	plan := NewScanPlan()
	for i := int64(1); i <= 3; i++ {
		repo := RepositoryRef{
			ID:   i,
			Name: fmt.Sprintf("repo%d", i),
			Repo: &github.Repository{
				ID:       github.Int64(i),
				Name:     github.String(fmt.Sprintf("repo%d", i)),
				Owner:    &github.User{Login: github.String("owner")},
				FullName: github.String(fmt.Sprintf("owner/repo%d", i)),
			},
		}
		rp := &RepoPlan{
			Repo: repo,
			Dependencies: map[data.DependencyKey]data.DependencyRequest{
				data.DepRepoMetadata:                       {Key: data.DepRepoMetadata},
				data.DepRepoDefaultBranchClassicProtection: {Key: data.DepRepoDefaultBranchClassicProtection},
			},
		}
		plan.RepoPlans[i] = rp
	}

	ctx := context.Background()
	resCh, errCh := scheduler.Execute(ctx, plan)

	got := make(map[int64]RepoExecutionResult)
	for res := range resCh {
		got[res.RepoID] = res
	}
	// errCh must close as well.
	for range errCh {
		// no fatal errors expected
	}

	if len(got) != 3 {
		t.Fatalf("Expected 3 streamed results, got %d", len(got))
	}
	for i := int64(1); i <= 3; i++ {
		res, ok := got[i]
		if !ok {
			t.Fatalf("Missing result for repo %d", i)
		}
		if _, ok := res.Data.Get(data.DepRepoMetadata); !ok {
			t.Fatalf("Repo %d missing metadata result", i)
		}
		if _, ok := res.Data.Get(data.DepRepoDefaultBranchClassicProtection); !ok {
			t.Fatalf("Repo %d missing branch protection result", i)
		}
		if len(res.DepErrs) != 0 {
			t.Fatalf("Repo %d expected no dependency errors, got %v", i, res.DepErrs)
		}
	}
}

func TestScheduler_Execute_Stream_CancellationStopsPromptly(t *testing.T) {
	mux := http.NewServeMux()

	// repo1 responds immediately
	mux.HandleFunc("/repos/owner/repo1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo1", "full_name":"owner/repo1", "default_branch":"main"}`)
	})
	// repo2 blocks until the client cancels
	mux.HandleFunc("/repos/owner/repo2", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	scheduler := newTestScheduler(t, mux, 2)

	plan := NewScanPlan()
	for i := int64(1); i <= 2; i++ {
		repo := RepositoryRef{
			ID:   i,
			Name: fmt.Sprintf("repo%d", i),
			Repo: &github.Repository{
				ID:       github.Int64(i),
				Name:     github.String(fmt.Sprintf("repo%d", i)),
				Owner:    &github.User{Login: github.String("owner")},
				FullName: github.String(fmt.Sprintf("owner/repo%d", i)),
			},
		}
		rp := &RepoPlan{
			Repo: repo,
			Dependencies: map[data.DependencyKey]data.DependencyRequest{
				data.DepRepoMetadata: {Key: data.DepRepoMetadata},
			},
		}
		plan.RepoPlans[i] = rp
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resCh, errCh := scheduler.Execute(ctx, plan)

	// Wait for first completion, then cancel.
	first, ok := <-resCh
	if !ok {
		t.Fatalf("Expected at least one result")
	}
	if first.RepoID != 1 {
		// With sorted repo IDs, repo1 should finish first.
		t.Fatalf("Expected first result for repo 1, got %d", first.RepoID)
	}
	cancel()

	// Drain result channel; should close without producing repo2.
	gotCount := 1
	for range resCh {
		gotCount++
	}
	if gotCount != 1 {
		t.Fatalf("Expected only 1 result after cancellation, got %d", gotCount)
	}

	// errCh should surface cancellation.
	var gotErr error
	for err := range errCh {
		gotErr = err
	}
	if gotErr == nil {
		t.Fatalf("Expected cancellation error on errCh")
	}
}

func TestScheduler_Execute_Stream_FailFastViaContextCancellation(t *testing.T) {
	// This test models fail-fast behavior implemented by callers: once a dependency
	// error is observed, cancel the context and ensure remaining repos never start.

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo1", "full_name":"owner/repo1", "default_branch":"main"}`)
	})
	mux.HandleFunc("/repos/owner/repo1/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"boom"}`)
	})
	mux.HandleFunc("/repos/owner/repo2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":2, "name":"repo2", "full_name":"owner/repo2", "default_branch":"main"}`)
	})
	mux.HandleFunc("/repos/owner/repo2/branches/main/protection", func(w http.ResponseWriter, r *http.Request) {
		// If this endpoint is ever hit, it means repo2 started.
		fmt.Fprint(w, `{"url": "protection_url"}`)
	})

	// Concurrency=1 ensures repo2 cannot start before we cancel.
	scheduler := newTestScheduler(t, mux, 1)

	plan := NewScanPlan()
	for i := int64(1); i <= 2; i++ {
		repo := RepositoryRef{
			ID:   i,
			Name: fmt.Sprintf("repo%d", i),
			Repo: &github.Repository{
				ID:       github.Int64(i),
				Name:     github.String(fmt.Sprintf("repo%d", i)),
				Owner:    &github.User{Login: github.String("owner")},
				FullName: github.String(fmt.Sprintf("owner/repo%d", i)),
			},
		}
		rp := &RepoPlan{
			Repo: repo,
			Dependencies: map[data.DependencyKey]data.DependencyRequest{
				data.DepRepoMetadata:                       {Key: data.DepRepoMetadata},
				data.DepRepoDefaultBranchClassicProtection: {Key: data.DepRepoDefaultBranchClassicProtection},
			},
		}
		plan.RepoPlans[i] = rp
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resCh, errCh := scheduler.Execute(ctx, plan)

	var got []RepoExecutionResult
	for res := range resCh {
		got = append(got, res)
		if len(res.DepErrs) > 0 {
			cancel()
		}
	}

	if len(got) != 1 {
		t.Fatalf("Expected exactly 1 result after fail-fast cancellation, got %d", len(got))
	}
	if got[0].RepoID != 1 {
		t.Fatalf("Expected only repo1 to complete, got repo %d", got[0].RepoID)
	}
	if got[0].DepErrs[data.DepRepoDefaultBranchClassicProtection] == nil {
		t.Fatalf("Expected dependency error recorded for branch protection")
	}

	var gotErr error
	for err := range errCh {
		gotErr = err
	}
	if gotErr == nil {
		t.Fatalf("Expected cancellation error on errCh")
	}
}

func TestScheduler_Execute_Stream_FatalNilRepoPlanDoesNotPanic(t *testing.T) {
	mux := http.NewServeMux()

	// repo1 responds immediately
	mux.HandleFunc("/repos/owner/repo1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo1", "full_name":"owner/repo1", "default_branch":"main"}`)
	})

	scheduler := newTestScheduler(t, mux, 2)

	plan := NewScanPlan()
	repo1 := RepositoryRef{
		ID:   1,
		Name: "repo1",
		Repo: &github.Repository{
			ID:       github.Int64(1),
			Name:     github.String("repo1"),
			Owner:    &github.User{Login: github.String("owner")},
			FullName: github.String("owner/repo1"),
		},
	}
	plan.RepoPlans[1] = &RepoPlan{
		Repo: repo1,
		Dependencies: map[data.DependencyKey]data.DependencyRequest{
			data.DepRepoMetadata: {Key: data.DepRepoMetadata},
		},
	}
	plan.RepoPlans[2] = nil

	ctx := context.Background()
	resCh, errCh := scheduler.Execute(ctx, plan)

	// Results may be 0 or 1 depending on timing, but the scheduler must close
	// channels cleanly and surface the fatal error.
	for range resCh {
		// drain
	}

	var gotErr error
	for err := range errCh {
		gotErr = err
	}
	if gotErr == nil {
		t.Fatalf("expected fatal error")
	}
	if !strings.Contains(gotErr.Error(), "nil repo plan") {
		t.Fatalf("expected nil repo plan error, got %v", gotErr)
	}
}
