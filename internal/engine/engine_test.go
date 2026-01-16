package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"repomedic/internal/config"
	"repomedic/internal/data"
	_ "repomedic/internal/fetcher/providers"
	gh "repomedic/internal/github"
	"repomedic/internal/output"
	"repomedic/internal/rules"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-github/v81/github"
)

type mockEvalRule struct {
	id      string
	called  bool
	success bool
}

func (r *mockEvalRule) ID() string          { return r.id }
func (r *mockEvalRule) Title() string       { return "Mock Eval Rule" }
func (r *mockEvalRule) Description() string { return "Mock Eval" }
func (r *mockEvalRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoMetadata}, nil
}
func (r *mockEvalRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	r.called = true
	val, ok := dc.Get(data.DepRepoMetadata)
	if ok && val != nil {
		r.success = true
		return rules.Result{Status: "PASS"}, nil
	}
	return rules.Result{Status: "FAIL"}, nil
}

type configurableToggleRule struct {
	id      string
	enabled bool
}

func (r *configurableToggleRule) ID() string          { return r.id }
func (r *configurableToggleRule) Title() string       { return "Test configurable toggle" }
func (r *configurableToggleRule) Description() string { return "Test-only configurable rule" }
func (r *configurableToggleRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoMetadata}, nil
}
func (r *configurableToggleRule) Options() []rules.Option {
	return []rules.Option{{
		Name:        "enabled",
		Description: "If true, the rule passes; if false, it fails.",
		Default:     "false",
	}}
}
func (r *configurableToggleRule) Configure(opts map[string]string) error {
	v, ok := opts["enabled"]
	if !ok || strings.TrimSpace(v) == "" {
		r.enabled = false
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("invalid value for enabled: %s", v)
	}
	r.enabled = b
	return nil
}
func (r *configurableToggleRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	if r.enabled {
		return rules.Result{Status: rules.StatusPass}, nil
	}
	return rules.Result{Status: rules.StatusFail, Message: "disabled"}, nil
}

func TestEngine_Run_EndToEnd(t *testing.T) {
	// Mock Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register Mock Rule
	ruleID := "test-eval-rule"
	mockRule := &mockEvalRule{id: ruleID}
	// TODO: Test hygiene: avoid mutating the global rules registry (or reset it via a test-only helper).
	// Today this registers into process-global state and relies on tests being isolated/order-independent.
	// Check if already registered to avoid panic if test runs multiple times in same process (unlikely here but good practice)
	// rules.Register panics if duplicate.
	// We can't easily check existence without List().
	// Let's just try to register and recover? Or assume it's fresh.
	// Or use a unique ID every time.
	// But Resolve needs to find it.

	// For now, just register.
	func() {
		defer func() { _ = recover() }() // Ignore panic if already registered
		rules.Register(mockRule)
	}()

	cfg := &config.Config{
		Targeting: config.Targeting{
			Repos: []string{"acme/repo"},
		},
		Rules: config.Rules{
			Selector: ruleID,
		},
		Runtime: config.Runtime{
			Concurrency: 1,
		},
	}

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify our registered rule instance was evaluated. (The rules registry stores the
	// `Rule` interface wrapping our `*mockEvalRule`, so field mutations are visible here.)

	if !mockRule.called {
		t.Error("Rule.Evaluate was not called")
	}
	if !mockRule.success {
		t.Error("Rule.Evaluate failed (missing data?)")
	}
}

func TestEngine_Run_FetchFailure(t *testing.T) {
	// Mock Server
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// Discovery success
			fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
		} else {
			// Fetch failure
			w.WriteHeader(500)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register Mock Rule
	ruleID := "test-fail-rule"
	mockRule := &mockEvalRule{id: ruleID}
	func() {
		defer func() { _ = recover() }()
		rules.Register(mockRule)
	}()

	cfg := &config.Config{
		Targeting: config.Targeting{
			Repos: []string{"acme/repo"},
		},
		Rules: config.Rules{
			Selector: ruleID,
		},
		Runtime: config.Runtime{
			Concurrency: 1,
		},
	}

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)

	if exitCode != 2 {
		t.Errorf("Expected exit code 2 (partial failure), got %d", exitCode)
	}

	// Verify rule was NOT called because dependency failed
	if mockRule.called {
		t.Error("Rule.Evaluate WAS called despite missing dependency")
	}
}

func TestEngine_Run_DependencyFailurePropagation_IncludesReason(t *testing.T) {
	// Mock Server
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// Discovery success
			fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
			return
		}
		// Fetch failure for repo.metadata
		w.WriteHeader(500)
		fmt.Fprint(w, `{"message":"boom"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register Mock Rule
	ruleID := "test-dep-fail-reason-rule"
	mockRule := &mockEvalRule{id: ruleID}
	func() {
		defer func() { _ = recover() }()
		rules.Register(mockRule)
	}()

	// Temp Output File to inspect emitted result.
	tmpFile, err := os.CreateTemp("", "engine_dep_fail_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	cfg := &config.Config{
		Targeting: config.Targeting{Repos: []string{"acme/repo"}},
		Rules:     config.Rules{Selector: ruleID},
		Output:    config.Output{Out: tmpPath, OutFormat: "json", NoConsole: true},
		Runtime:   config.Runtime{Concurrency: 1},
	}

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)
	if exitCode != 2 {
		t.Fatalf("Expected exit code 2 (partial failure), got %d", exitCode)
	}

	// Verify rule was NOT called because dependency failed
	if mockRule.called {
		t.Fatalf("Rule.Evaluate WAS called despite dependency failure")
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var results []rules.Result
	if err := json.Unmarshal(content, &results); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Status != rules.StatusError {
		t.Fatalf("Expected StatusError, got %s", results[0].Status)
	}
	if !strings.Contains(results[0].Message, "500") {
		t.Fatalf("Expected message to include failure reason (HTTP 500), got %q", results[0].Message)
	}
}

func TestEngine_Run_FileOutput(t *testing.T) {
	// Mock Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register Mock Rule
	ruleID := "test-output-rule"
	mockRule := &mockEvalRule{id: ruleID}
	func() {
		defer func() { _ = recover() }()
		rules.Register(mockRule)
	}()

	// Temp Output File
	tmpFile, err := os.CreateTemp("", "engine_output_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cfg := &config.Config{
		Targeting: config.Targeting{
			Repos: []string{"acme/repo"},
		},
		Rules: config.Rules{
			Selector: ruleID,
		},
		Output: config.Output{
			Out:       tmpPath,
			OutFormat: "json",
			NoConsole: true,
		},
		Runtime: config.Runtime{
			Concurrency: 1,
		},
	}

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Verify File Content
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var results []rules.Result
	if err := json.Unmarshal(content, &results); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].RuleID != ruleID {
		t.Errorf("Expected rule ID %s, got %s", ruleID, results[0].RuleID)
	}
}

type alwaysFailRule struct {
	id string
}

func (r *alwaysFailRule) ID() string          { return r.id }
func (r *alwaysFailRule) Title() string       { return "Always Fail Rule" }
func (r *alwaysFailRule) Description() string { return "Always fails" }
func (r *alwaysFailRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoMetadata}, nil
}
func (r *alwaysFailRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	return rules.Result{Status: rules.StatusFail}, nil
}

func TestEngine_Run_ExitCodeIs1WhenWrongsDetected(t *testing.T) {
	// Mock Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register rule
	ruleID := "test-always-fail-rule"
	func() {
		defer func() { _ = recover() }()
		rules.Register(&alwaysFailRule{id: ruleID})
	}()

	cfg := &config.Config{
		Targeting: config.Targeting{Repos: []string{"acme/repo"}},
		Rules:     config.Rules{Selector: ruleID},
		Output:    config.Output{NoConsole: true},
		Runtime:   config.Runtime{Concurrency: 1},
	}

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)

	if exitCode != 1 {
		t.Fatalf("Expected exit code 1 (wrongs detected), got %d", exitCode)
	}
}

func TestEngine_Run_ExitCodeIs3OnFatalDiscoveryError(t *testing.T) {
	// Mock Server returns 500 for discovery
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register rule
	ruleID := "test-fatal-discovery-rule"
	func() {
		defer func() { _ = recover() }()
		rules.Register(&alwaysFailRule{id: ruleID})
	}()

	cfg := &config.Config{
		Targeting: config.Targeting{Repos: []string{"acme/repo"}},
		Rules:     config.Rules{Selector: ruleID},
		Output:    config.Output{NoConsole: true},
		Runtime:   config.Runtime{Concurrency: 1},
	}

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)

	if exitCode != 3 {
		t.Fatalf("Expected exit code 3 (fatal error), got %d", exitCode)
	}
}

type recordingNoDepsRule struct {
	id    string
	repos []string
}

func (r *recordingNoDepsRule) ID() string          { return r.id }
func (r *recordingNoDepsRule) Title() string       { return "Recording NoDeps Rule" }
func (r *recordingNoDepsRule) Description() string { return "Records repos evaluated" }
func (r *recordingNoDepsRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return nil, nil
}
func (r *recordingNoDepsRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	r.repos = append(r.repos, repo.GetFullName())
	return rules.Result{Status: rules.StatusPass}, nil
}

func TestEngine_Run_AppliesFiltering_BeforePlanning(t *testing.T) {
	// Mock Server for org discovery (filtering applies after discovery)
	mux := http.NewServeMux()
	mux.HandleFunc("/orgs/acme/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{"id":1, "name":"a", "full_name":"acme/a", "default_branch":"main", "owner":{"login":"acme"}, "topics":["go"], "archived":false, "fork":false, "visibility":"public"},
			{"id":2, "name":"b", "full_name":"acme/b", "default_branch":"main", "owner":{"login":"acme"}, "topics":["go"], "archived":false, "fork":false, "visibility":"public"},
			{"id":3, "name":"c", "full_name":"acme/c", "default_branch":"main", "owner":{"login":"acme"}, "topics":["go"], "archived":true, "fork":false, "visibility":"public"}
		]`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	ruleID := "test-filtering-record-rule"
	recorder := &recordingNoDepsRule{id: ruleID}
	func() {
		defer func() { _ = recover() }()
		rules.Register(recorder)
	}()

	cfg := config.New()
	cfg.Targeting.Org = "acme"
	cfg.Targeting.Topic = []string{"go"}
	cfg.Targeting.Exclude = []string{"acme/b"}
	// archived default is exclude; repo c is archived and must be filtered out
	cfg.Rules.Selector = ruleID
	cfg.Output.NoConsole = true
	cfg.Runtime.Concurrency = 1

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if len(recorder.repos) != 1 || recorder.repos[0] != "acme/a" {
		t.Fatalf("expected only acme/a to be evaluated after filtering, got %v", recorder.repos)
	}
}

func TestEngine_Run_ExplicitRepos_BypassesFiltering(t *testing.T) {
	// If a repo is explicitly provided via --repos (including URL form), it must be scanned
	// even if it is archived/forked (defaults would otherwise filter it out).
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/archived-fork", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":9, "name":"archived-fork", "full_name":"acme/archived-fork", "default_branch":"main", "owner":{"login":"acme"}, "archived":true, "fork":true, "visibility":"public"}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	cfg := config.New()
	cfg.Targeting.Repos = []string{"https://github.com/acme/archived-fork"}
	cfg.Targeting.DryRun = true
	cfg.Runtime.Concurrency = 1

	// Capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w
	os.Stderr = w

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)

	_ = w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	out := buf.String()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for dry-run, got %d; output=%s", exitCode, out)
	}
	if !strings.Contains(out, "Found 1 repositories.") {
		t.Fatalf("expected explicit repo to bypass filtering; output=%s", out)
	}
	if !strings.Contains(out, "acme/archived-fork") {
		t.Fatalf("expected explicit repo to be listed in dry-run; output=%s", out)
	}
}

func TestEngine_Run_DryRun_PrintsDeterministicRepoSet_AndCreatesNoArtifacts(t *testing.T) {
	// Mock Server for resolving repos
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/b", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":2, "name":"b", "full_name":"acme/b", "default_branch":"main", "owner":{"login":"acme"}}`)
	})
	mux.HandleFunc("/repos/acme/a", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"a", "full_name":"acme/a", "default_branch":"main", "owner":{"login":"acme"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Choose output paths that must NOT be created in dry-run.
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "results.ndjson")
	reportPath := filepath.Join(tmpDir, "report.md")

	cfg := config.New()
	cfg.Targeting.Repos = []string{"acme/b", "acme/a"} // intentionally unsorted
	cfg.Targeting.DryRun = true
	cfg.Output.Out = outPath
	cfg.Output.OutFormat = "ndjson"
	cfg.Output.Report = reportPath
	cfg.Runtime.Concurrency = 1

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	_ = r.Close()
	out := buf.String()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for dry-run, got %d; output=%s", exitCode, out)
	}

	if !strings.Contains(out, "Resolved repositories:") {
		t.Fatalf("expected dry-run to print resolved repo header; output=%s", out)
	}

	// Expect sorted order: acme/a then acme/b
	idxA := strings.Index(out, "acme/a")
	idxB := strings.Index(out, "acme/b")
	if idxA == -1 || idxB == -1 || idxA > idxB {
		t.Fatalf("expected deterministic sorted repo list (acme/a before acme/b); output=%s", out)
	}

	if _, err := os.Stat(outPath); err == nil {
		t.Fatalf("expected no structured output file in dry-run, but %s exists", outPath)
	}
	if _, err := os.Stat(reportPath); err == nil {
		t.Fatalf("expected no report file in dry-run, but %s exists", reportPath)
	}
}

func TestEngine_Run_SetRuleOptions_ChangesBehavior(t *testing.T) {
	// Register a deterministic configurable rule to validate --set routing.
	toggle := &configurableToggleRule{id: "test-configurable-toggle"}
	func() {
		defer func() { _ = recover() }()
		rules.Register(toggle)
	}()

	// Mock Server for discovery
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	eng := NewEngine(ghClient)

	// Helper to run and return single result status.
	run := func(t *testing.T, set []string) rules.Status {
		t.Helper()
		tmpFile, err := os.CreateTemp("", "engine_set_opts_*.json")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		_ = tmpFile.Close()
		defer os.Remove(tmpPath)

		cfg := config.New()
		cfg.Targeting.Repos = []string{"acme/repo"}
		cfg.Rules.Selector = toggle.ID()
		cfg.Rules.Set = set
		cfg.Output.Out = tmpPath
		cfg.Output.OutFormat = "json"
		cfg.Output.NoConsole = true
		cfg.Runtime.Concurrency = 1

		exitCode := eng.Run(context.Background(), cfg)
		if exitCode == 3 {
			t.Fatalf("unexpected fatal exit code 3")
		}

		content, err := os.ReadFile(tmpPath)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}
		var results []rules.Result
		if err := json.Unmarshal(content, &results); err != nil {
			t.Fatalf("Failed to unmarshal output: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		return results[0].Status
	}

	// Default is enabled=false -> FAIL.
	statusDefault := run(t, nil)
	if statusDefault != rules.StatusFail {
		t.Fatalf("expected default to FAIL, got %s", statusDefault)
	}

	// With enabled=true, it should PASS.
	statusConfigured := run(t, []string{toggle.ID() + ".enabled=true"})
	if statusConfigured != rules.StatusPass {
		t.Fatalf("expected configured to PASS, got %s", statusConfigured)
	}

	// Reset global rule state to default for safety.
	_ = toggle.Configure(map[string]string{"enabled": "false"})
}

func TestEngine_Run_NDJSON_EmitsLifecycleEvents(t *testing.T) {
	// Mock Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register a deterministic rule.
	ruleID := "test-ndjson-events-rule"
	func() {
		defer func() { _ = recover() }()
		rules.Register(&alwaysFailRule{id: ruleID})
	}()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "events.ndjson")

	cfg := config.New()
	cfg.Targeting.Repos = []string{"acme/repo"}
	cfg.Rules.Selector = ruleID
	cfg.Output.Out = outPath
	cfg.Output.OutFormat = "ndjson"
	cfg.Output.NoConsole = true
	cfg.Runtime.Concurrency = 1

	eng := NewEngine(ghClient)
	exitCode := eng.Run(context.Background(), cfg)
	if exitCode == 3 {
		t.Fatalf("unexpected fatal exit code 3")
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read ndjson output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected ndjson output lines")
	}

	required := map[string]bool{
		"run.started":   false,
		"repo.started":  false,
		"rule.result":   false,
		"repo.finished": false,
		"run.finished":  false,
	}

	for _, line := range lines {
		var ev output.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("invalid json line %q: %v", line, err)
		}
		if _, ok := required[ev.Type]; ok {
			required[ev.Type] = true
		}
	}

	for typ, seen := range required {
		if !seen {
			t.Fatalf("missing required event type %q", typ)
		}
	}
}

func TestEngine_Run_NDJSON_LifecycleEventOrdering(t *testing.T) {
	// This is a minimal ordering contract check:
	// - run.started must be the first emitted event
	// - run.finished must be the last emitted event

	// Mock Server
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo", "full_name":"acme/repo", "default_branch":"main", "owner":{"login":"acme"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Register a deterministic rule.
	ruleID := "test-ndjson-order-rule"
	func() {
		defer func() { _ = recover() }()
		rules.Register(&alwaysFailRule{id: ruleID})
	}()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "events.ndjson")

	cfg := config.New()
	cfg.Targeting.Repos = []string{"acme/repo"}
	cfg.Rules.Selector = ruleID
	cfg.Output.Out = outPath
	cfg.Output.OutFormat = "ndjson"
	cfg.Output.NoConsole = true
	cfg.Runtime.Concurrency = 1

	eng := NewEngine(ghClient)
	_ = eng.Run(context.Background(), cfg)

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read ndjson output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 ndjson lines, got %d", len(lines))
	}

	var first output.Event
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("invalid first json line %q: %v", lines[0], err)
	}
	if first.Type != "run.started" {
		t.Fatalf("expected first event type run.started, got %q", first.Type)
	}

	var last output.Event
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("invalid last json line %q: %v", lines[len(lines)-1], err)
	}
	if last.Type != "run.finished" {
		t.Fatalf("expected last event type run.finished, got %q", last.Type)
	}
}

func TestEngine_Run_NDJSON_PerRepoOrdering(t *testing.T) {
	// Contract:
	// - run.started first, run.finished last
	// - for each repo: repo.started occurs before any rule.result for that repo
	// - for each repo: repo.finished occurs after all rule.result for that repo

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/acme/repo1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":1, "name":"repo1", "full_name":"acme/repo1", "default_branch":"main", "owner":{"login":"acme"}}`)
	})
	mux.HandleFunc("/repos/acme/repo2", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":2, "name":"repo2", "full_name":"acme/repo2", "default_branch":"main", "owner":{"login":"acme"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	u, _ := url.Parse(server.URL + "/")
	client.BaseURL = u
	ghClient := &gh.Client{Client: client}

	// Deterministic rule.
	ruleID := "test-ndjson-per-repo-order-rule"
	func() {
		defer func() { _ = recover() }()
		rules.Register(&alwaysFailRule{id: ruleID})
	}()

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "events.ndjson")

	cfg := config.New()
	cfg.Targeting.Repos = []string{"acme/repo1", "acme/repo2"}
	cfg.Rules.Selector = ruleID
	cfg.Output.Out = outPath
	cfg.Output.OutFormat = "ndjson"
	cfg.Output.NoConsole = true
	cfg.Runtime.Concurrency = 2

	eng := NewEngine(ghClient)
	_ = eng.Run(context.Background(), cfg)

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read ndjson output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 ndjson lines, got %d", len(lines))
	}

	// Parse events.
	events := make([]output.Event, 0, len(lines))
	for _, line := range lines {
		var ev output.Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("invalid json line %q: %v", line, err)
		}
		events = append(events, ev)
	}

	if events[0].Type != "run.started" {
		t.Fatalf("expected first event run.started, got %q", events[0].Type)
	}
	if events[len(events)-1].Type != "run.finished" {
		t.Fatalf("expected last event run.finished, got %q", events[len(events)-1].Type)
	}

	seenStarted := map[string]bool{"acme/repo1": false, "acme/repo2": false}
	seenFinished := map[string]bool{"acme/repo1": false, "acme/repo2": false}

	for _, ev := range events {
		if ev.Repo == "" {
			continue
		}
		switch ev.Type {
		case "repo.started":
			seenStarted[ev.Repo] = true
			if seenFinished[ev.Repo] {
				t.Fatalf("repo.started occurred after repo.finished for %s", ev.Repo)
			}
		case "rule.result":
			if !seenStarted[ev.Repo] {
				t.Fatalf("rule.result before repo.started for %s", ev.Repo)
			}
			if seenFinished[ev.Repo] {
				t.Fatalf("rule.result after repo.finished for %s", ev.Repo)
			}
		case "repo.finished":
			if !seenStarted[ev.Repo] {
				t.Fatalf("repo.finished before repo.started for %s", ev.Repo)
			}
			seenFinished[ev.Repo] = true
		}
	}

	for repo, ok := range seenStarted {
		if !ok {
			t.Fatalf("missing repo.started for %s", repo)
		}
	}
	for repo, ok := range seenFinished {
		if !ok {
			t.Fatalf("missing repo.finished for %s", repo)
		}
	}
}

type signalOnFirstEvalRule struct {
	id     string
	hit    chan<- struct{}
	mu     sync.Mutex
	called int
}

func (r *signalOnFirstEvalRule) ID() string          { return r.id }
func (r *signalOnFirstEvalRule) Title() string       { return "Signal Rule" }
func (r *signalOnFirstEvalRule) Description() string { return "Signals on first Evaluate call" }
func (r *signalOnFirstEvalRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return []data.DependencyKey{data.DepRepoMetadata}, nil
}
func (r *signalOnFirstEvalRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	r.mu.Lock()
	r.called++
	called := r.called
	r.mu.Unlock()
	if called == 1 {
		r.hit <- struct{}{}
	}
	return rules.Result{Status: rules.StatusPass}, nil
}

func TestEvaluateStreamingResults_StreamsPerRepoCompletion(t *testing.T) {
	// This is a deterministic streaming behavior test:
	// send repo1 result, assert repo1 events emitted before repo2 result is sent.

	cfg := config.New()
	cfg.Runtime.Verbose = false

	hit := make(chan struct{}, 1)
	rule := &signalOnFirstEvalRule{id: "signal-rule", hit: hit}

	repo1 := RepositoryRef{Owner: "acme", Name: "repo1", ID: 1, Repo: &github.Repository{ID: github.Ptr(int64(1))}}
	repo2 := RepositoryRef{Owner: "acme", Name: "repo2", ID: 2, Repo: &github.Repository{ID: github.Ptr(int64(2))}}
	plan := NewScanPlan()
	plan.RepoPlans[1] = &RepoPlan{Repo: repo1, Rules: []rules.Rule{rule}}
	plan.RepoPlans[2] = &RepoPlan{Repo: repo2, Rules: []rules.Rule{rule}}

	var buf bytes.Buffer
	outMgr := output.NewManager()
	if err := outMgr.AddSink(output.NewConsoleSink(&buf, "ndjson", nil)); err != nil {
		t.Fatalf("AddSink error: %v", err)
	}

	resCh := make(chan RepoExecutionResult)
	done := make(chan struct{})
	go func() {
		defer close(done)
		evaluateStreamingResults(context.Background(), cfg, plan, resCh, outMgr)
	}()

	resCh <- RepoExecutionResult{
		RepoID: 1,
		Data: data.NewMapDataContext(map[data.DependencyKey]any{
			data.DepRepoMetadata: &github.Repository{ID: github.Ptr(int64(1))},
		}),
		DepErrs: map[data.DependencyKey]error{},
	}

	select {
	case <-hit:
		// repo1 evaluated
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for first Evaluate call")
	}

	// Wait for repo.finished to appear in output (Evaluate returns -> Write result -> Write repo.finished)
	// Since hit is sent *during* Evaluate, we need to give the engine a moment to finish up.
	var soFar string
	ok := false
	for i := 0; i < 10; i++ {
		soFar = buf.String()
		if strings.Contains(soFar, `"type":"repo.finished"`) && strings.Contains(soFar, `"repo":"acme/repo1"`) {
			ok = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ok {
		t.Fatalf("expected repo1.finished in output, got: %s", soFar)
	}

	if !strings.Contains(soFar, `"type":"repo.started"`) || !strings.Contains(soFar, `"repo":"acme/repo1"`) {
		t.Fatalf("expected repo1.started in output, got: %s", soFar)
	}
	if strings.Contains(soFar, "acme/repo2") {
		t.Fatalf("expected no repo2 output before repo2 result arrives, got: %s", soFar)
	}

	resCh <- RepoExecutionResult{
		RepoID: 2,
		Data: data.NewMapDataContext(map[data.DependencyKey]any{
			data.DepRepoMetadata: &github.Repository{ID: github.Ptr(int64(2))},
		}),
		DepErrs: map[data.DependencyKey]error{},
	}
	close(resCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for streaming evaluation to finish")
	}

	final := buf.String()
	if !strings.Contains(final, "acme/repo2") {
		t.Fatalf("expected repo2 events in output, got: %s", final)
	}
}

type undeclaredDepAccessRule struct {
	id string
}

func (r *undeclaredDepAccessRule) ID() string    { return r.id }
func (r *undeclaredDepAccessRule) Title() string { return "Undeclared Dep Access" }
func (r *undeclaredDepAccessRule) Description() string {
	return "Reads a dependency key it did not declare"
}
func (r *undeclaredDepAccessRule) Dependencies(ctx context.Context, repo *github.Repository) ([]data.DependencyKey, error) {
	return nil, nil
}
func (r *undeclaredDepAccessRule) Evaluate(ctx context.Context, repo *github.Repository, dc data.DataContext) (rules.Result, error) {
	_, _ = dc.Get(data.DepRepoMetadata)
	return rules.Result{Status: rules.StatusPass}, nil
}

func TestEvaluateStreamingResults_ErrorsOnUndeclaredDependencyAccess(t *testing.T) {
	cfg := config.New()
	cfg.Runtime.Verbose = false

	rule := &undeclaredDepAccessRule{id: "undeclared-dep"}
	repo := RepositoryRef{Owner: "acme", Name: "repo", ID: 1, Repo: &github.Repository{ID: github.Ptr(int64(1))}}
	plan := NewScanPlan()
	plan.RepoPlans[1] = &RepoPlan{Repo: repo, Rules: []rules.Rule{rule}}

	var buf bytes.Buffer
	outMgr := output.NewManager()
	if err := outMgr.AddSink(output.NewConsoleSink(&buf, "ndjson", nil)); err != nil {
		t.Fatalf("AddSink error: %v", err)
	}

	resCh := make(chan RepoExecutionResult)
	done := make(chan struct{})
	go func() {
		defer close(done)
		evaluateStreamingResults(context.Background(), cfg, plan, resCh, outMgr)
	}()

	resCh <- RepoExecutionResult{
		RepoID: 1,
		Data: data.NewMapDataContext(map[data.DependencyKey]any{
			data.DepRepoMetadata: &github.Repository{ID: github.Ptr(int64(1))},
		}),
		DepErrs: map[data.DependencyKey]error{},
	}
	close(resCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for streaming evaluation to finish")
	}

	final := buf.String()
	if !strings.Contains(final, `"type":"rule.result"`) {
		t.Fatalf("expected rule.result event, got: %s", final)
	}
	if !strings.Contains(final, `"rule_id":"undeclared-dep"`) {
		t.Fatalf("expected rule_id in output, got: %s", final)
	}
	if !strings.Contains(final, `"status":"ERROR"`) {
		t.Fatalf("expected ERROR status, got: %s", final)
	}
	if !strings.Contains(final, "undeclared") {
		t.Fatalf("expected undeclared dependency message, got: %s", final)
	}
}

func TestRuleResultIfDependenciesMissingOrFailed_SingleFailureDropsKeyPrefix(t *testing.T) {
	dc := data.NewMapDataContext(map[data.DependencyKey]any{})

	repoDepErrs := map[data.DependencyKey]error{
		data.DepRepoMetadata: fmt.Errorf("GET https://api.github.com/x: 500 boom"),
	}

	status, msg, ok := ruleResultIfDependenciesMissingOrFailed(dc, []data.DependencyKey{data.DepRepoMetadata}, repoDepErrs, false)
	if !ok {
		t.Fatalf("expected dependency result")
	}
	if status != rules.StatusError {
		t.Fatalf("expected StatusError, got %s", status)
	}
	if strings.Contains(msg, "repo.metadata") {
		t.Fatalf("expected single failure message to drop key prefix, got %q", msg)
	}
}

func TestRuleResultIfDependenciesMissingOrFailed_MultipleFailuresIncludesKeyPrefixes(t *testing.T) {
	dc := data.NewMapDataContext(map[data.DependencyKey]any{})

	repoDepErrs := map[data.DependencyKey]error{
		data.DepRepoMetadata:                       fmt.Errorf("GET https://api.github.com/x: 500 boom"),
		data.DepRepoDefaultBranchClassicProtection: fmt.Errorf("GET https://api.github.com/y: 500 boom"),
	}

	status, msg, ok := ruleResultIfDependenciesMissingOrFailed(
		dc,
		[]data.DependencyKey{data.DepRepoMetadata, data.DepRepoDefaultBranchClassicProtection},
		repoDepErrs,
		false,
	)
	if !ok {
		t.Fatalf("expected dependency result")
	}
	if status != rules.StatusError {
		t.Fatalf("expected StatusError, got %s", status)
	}
	if !strings.Contains(msg, string(data.DepRepoMetadata)+": ") || !strings.Contains(msg, string(data.DepRepoDefaultBranchClassicProtection)+": ") {
		t.Fatalf("expected multi-failure message to include key prefixes, got %q", msg)
	}
}
