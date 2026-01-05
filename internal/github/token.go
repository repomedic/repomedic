package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

type AuthTokenSource string

const (
	AuthTokenSourceExplicit AuthTokenSource = "explicit"
	AuthTokenSourceEnv      AuthTokenSource = "env:GITHUB_TOKEN"
	AuthTokenSourceGitHubCL AuthTokenSource = "gh"
)

// ResolveAuthToken resolves a GitHub access token.
//
// Precedence:
//  1. provided (if non-empty)
//  2. GITHUB_TOKEN env var
//  3. GitHub CLI: `gh auth token -h github.com`
//
// It never prints the token.
func ResolveAuthToken(ctx context.Context, provided string) (token string, source AuthTokenSource, err error) {
	if tok := strings.TrimSpace(provided); tok != "" {
		return tok, AuthTokenSourceExplicit, nil
	}

	if env := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); env != "" {
		return env, AuthTokenSourceEnv, nil
	}

	tok, ok, err := tokenFromGitHubCLI(ctx)
	if err != nil {
		return "", "", err
	}
	if ok {
		return tok, AuthTokenSourceGitHubCL, nil
	}
	return "", "", nil
}

func tokenFromGitHubCLI(ctx context.Context) (token string, ok bool, err error) {
	_, lookErr := exec.LookPath("gh")
	if lookErr != nil {
		return "", false, nil
	}

	// Keep this bounded so a broken gh config or credential helper
	// doesn't hang scans.
	cmdCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(cmdCtx, "gh", "auth", "token", "-h", "github.com")
	// Ensure GH_PAGER is set deterministically (no duplicates).
	env := os.Environ()
	filteredEnv := env[:0]
	for _, entry := range env {
		if strings.HasPrefix(entry, "GH_PAGER=") {
			continue
		}
		filteredEnv = append(filteredEnv, entry)
	}
	cmd.Env = append(filteredEnv, "GH_PAGER=cat")
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		// If the context was canceled or timed out, surface that to callers.
		if cmdCtx.Err() != nil {
			return "", false, cmdCtx.Err()
		}
		// If gh is present but not logged in, or otherwise fails, treat as "no token".
		// We don't surface the raw gh output to avoid leaking any sensitive context.
		return "", false, nil
	}

	tok := strings.TrimSpace(string(out))
	if tok == "" {
		return "", false, nil
	}

	// Basic sanity: tokens must not contain whitespace.
	if strings.ContainsAny(tok, " \t\n\r") {
		return "", false, errors.New("invalid token returned by gh: contains whitespace")
	}

	return tok, true, nil
}
