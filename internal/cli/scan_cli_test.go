package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withoutEnv(key string) []string {
	out := make([]string, 0, len(os.Environ()))
	prefix := key + "="
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	// internal/cli -> repo root
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func goExe() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

func buildRepoMedicBinary(t *testing.T) string {
	t.Helper()

	outPath := filepath.Join(t.TempDir(), "repomedic-test")
	if runtime.GOOS == "windows" {
		outPath += ".exe"
	}

	cmd := exec.Command(goExe(), "build", "-o", outPath, "./cmd/repomedic")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build repomedic binary: %v; output=%s", err, string(out))
	}

	return outPath
}

func TestScan_ExitCode3_WhenNoScopeProvided(t *testing.T) {
	binary := buildRepoMedicBinary(t)
	// Pass a flag (e.g. --verbose) to bypass the "print help if no flags" check
	// and force the validation logic to run (and fail due to missing scope).
	cmd := exec.Command(binary, "scan", "--verbose")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit; output=%s", string(out))
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v; output=%s", err, err, string(out))
	}
	if code := exitErr.ProcessState.ExitCode(); code != 3 {
		t.Fatalf("expected exit code 3, got %d; output=%s", code, string(out))
	}
	if !strings.Contains(string(out), "at least one of --org, --user, --enterprise, or --repos must be provided") {
		t.Fatalf("expected validation message; output=%s", string(out))
	}
}

func TestScan_ExitCode3_WhenOutFormatCannotBeInferred(t *testing.T) {
	binary := buildRepoMedicBinary(t)
	cmd := exec.Command(binary, "scan", "--repos", "acme/foo", "--out", "results.unknown")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit; output=%s", string(out))
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v; output=%s", err, err, string(out))
	}
	if code := exitErr.ProcessState.ExitCode(); code != 3 {
		t.Fatalf("expected exit code 3, got %d; output=%s", code, string(out))
	}
	if !strings.Contains(string(out), "cannot infer output format") {
		t.Fatalf("expected output format inference error; output=%s", string(out))
	}
}

func TestScan_DryRun_DoesNotPrintConfigJSON(t *testing.T) {
	// Use enterprise scope to avoid any GitHub network access:
	// discovery will fail fast with a known error, but we still validate that
	// the CLI no longer dumps the parsed config JSON before running the engine.
	binary := buildRepoMedicBinary(t)
	cmd := exec.Command(binary, "scan", "--enterprise", "dummy-enterprise", "--dry-run")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit (enterprise not implemented); output=%s", string(out))
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v; output=%s", err, err, string(out))
	}
	if code := exitErr.ProcessState.ExitCode(); code != 3 {
		t.Fatalf("expected exit code 3, got %d; output=%s", code, string(out))
	}

	// Previously, `--dry-run` printed the full config JSON (containing "Targeting").
	if strings.Contains(string(out), "\"Targeting\"") {
		t.Fatalf("expected dry-run to not print config JSON; output=%s", string(out))
	}
}

func TestScan_ExitCode3_WhenGitHubTokenMissing(t *testing.T) {
	binary := buildRepoMedicBinary(t)
	cmd := exec.Command(binary, "scan", "--repos", "acme/foo", "--dry-run")
	// Ensure we don't accidentally pick up a developer's GitHub CLI session.
	// The scan command will attempt `gh auth token` as a fallback.
	cmd.Env = append(withoutEnv("GITHUB_TOKEN"), "PATH="+t.TempDir())

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit; output=%s", string(out))
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v; output=%s", err, err, string(out))
	}
	if code := exitErr.ProcessState.ExitCode(); code != 3 {
		t.Fatalf("expected exit code 3, got %d; output=%s", code, string(out))
	}
	if !strings.Contains(string(out), "GitHub auth token is required") {
		t.Fatalf("expected token-required message; output=%s", string(out))
	}
}

func TestScan_Help_DocumentsOutputAndExitCodes(t *testing.T) {
	binary := buildRepoMedicBinary(t)
	cmd := exec.Command(binary, "scan", "--help")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected zero exit; err=%v; output=%s", err, string(out))
	}

	s := string(out)
	// Regression guard: command help must remain agent-friendly and document
	// machine-readable output + exit status semantics.
	required := []string{
		"Output:",
		"Exit codes:",
		"NDJSON mode emits",
		"run.started",
		"rule.result",
		"run.finished",
	}
	for _, r := range required {
		if !strings.Contains(s, r) {
			t.Fatalf("expected scan --help to contain %q; output=%s", r, s)
		}
	}
}
