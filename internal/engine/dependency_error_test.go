package engine

import (
	"net/http"
	"testing"

	"github.com/google/go-github/v66/github"

	"repomedic/internal/data"
)

func TestPresentDependencyError_Forbidden_DefaultBranchProtection_IsSkippableAndPreservesMessage(t *testing.T) {
	msg := "Upgrade to GitHub Pro or make this repository public to enable this feature."
	err := &github.ErrorResponse{
		Response: &http.Response{StatusCode: 403, Status: "403 Forbidden"},
		Message:  msg,
	}

	pres := presentDependencyError(data.DepRepoDefaultBranchClassicProtection, err, false)
	if pres.disposition != depErrDispositionSkip {
		t.Fatalf("expected skippable disposition, got %v", pres.disposition)
	}
	if pres.message != msg {
		t.Fatalf("expected message %q, got %q", msg, pres.message)
	}
}

func TestPresentDependencyError_Forbidden_Metadata_IsHardError(t *testing.T) {
	err := &github.ErrorResponse{
		Response: &http.Response{StatusCode: 403, Status: "403 Forbidden"},
		Message:  "Resource not accessible by integration",
	}

	pres := presentDependencyError(data.DepRepoMetadata, err, false)
	if pres.disposition != depErrDispositionError {
		t.Fatalf("expected hard error disposition, got %v", pres.disposition)
	}
	if pres.message == "" {
		t.Fatalf("expected non-empty message")
	}
}

func TestScrubGitHubRequestFromErrorString_StripsURLPrefix(t *testing.T) {
	s := "GET https://api.github.com/repos/acme/foo/branches/main/protection: 403 some message []"
	out := scrubGitHubRequestFromErrorString(s)
	if out == "" {
		t.Fatalf("expected scrubbed output")
	}
	if out == s {
		t.Fatalf("expected output to differ from input")
	}
	if want := "403 some message []"; out != want {
		t.Fatalf("expected %q, got %q", want, out)
	}
}
