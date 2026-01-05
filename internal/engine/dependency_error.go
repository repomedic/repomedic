package engine

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v66/github"

	"repomedic/internal/data"
)

type depErrorDisposition int

const (
	depErrDispositionError depErrorDisposition = iota
	depErrDispositionSkip
)

type depErrorPresentation struct {
	disposition depErrorDisposition
	message     string
	verbose     string
}

func isSkippableForbidden(key data.DependencyKey) bool {
	switch key {
	case data.DepRepoDefaultBranchClassicProtection:
		return true
	default:
		return false
	}
}

func presentDependencyError(key data.DependencyKey, err error, verbose bool) depErrorPresentation {
	if err == nil {
		return depErrorPresentation{disposition: depErrDispositionError, message: "unknown error"}
	}

	full := err.Error()

	// Prefer structured GitHub error types to avoid leaking full request URLs.
	var er *github.ErrorResponse
	if errors.As(err, &er) {
		msg := strings.TrimSpace(er.Message)
		if !verbose && er.Response != nil && er.Response.StatusCode == http.StatusForbidden && isSkippableForbidden(key) {
			if msg == "" {
				msg = "GitHub API request forbidden"
			}
			return depErrorPresentation{disposition: depErrDispositionSkip, message: msg}
		}

		if verbose {
			return depErrorPresentation{disposition: depErrDispositionError, message: full, verbose: full}
		}

		status := ""
		if er.Response != nil {
			status = fmt.Sprintf("%d %s", er.Response.StatusCode, http.StatusText(er.Response.StatusCode))
		}
		if msg == "" {
			msg = "GitHub API request failed"
		}
		if status != "" {
			return depErrorPresentation{disposition: depErrDispositionError, message: fmt.Sprintf("GitHub API request failed (%s): %s", status, msg)}
		}
		return depErrorPresentation{disposition: depErrDispositionError, message: fmt.Sprintf("GitHub API request failed: %s", msg)}
	}

	// Fallback: best-effort scrub to avoid printing full request details.
	s := strings.TrimSpace(full)
	if verbose {
		return depErrorPresentation{disposition: depErrDispositionError, message: full, verbose: full}
	}
	if scrubbed := scrubGitHubRequestFromErrorString(s); scrubbed != "" {
		return depErrorPresentation{disposition: depErrDispositionError, message: scrubbed}
	}
	return depErrorPresentation{disposition: depErrDispositionError, message: "GitHub API request failed"}
}

func scrubGitHubRequestFromErrorString(s string) string {
	// Typical go-github error format:
	//   GET https://api.github.com/...: 403 Some message. [..]
	// We want to drop the leading "GET https://...: " part.
	methods := []string{"GET ", "POST ", "PUT ", "PATCH ", "DELETE "}
	for _, m := range methods {
		if strings.HasPrefix(s, m) {
			if i := strings.Index(s, "https://"); i >= 0 {
				if j := strings.Index(s[i:], ": "); j >= 0 {
					out := strings.TrimSpace(s[i+j+2:])
					return out
				}
			}
			if j := strings.Index(s, ": "); j >= 0 {
				return strings.TrimSpace(s[j+2:])
			}
			break
		}
	}
	return ""
}
