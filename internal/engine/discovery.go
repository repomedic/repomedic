package engine

import (
	"context"
	"fmt"
	"net/url"
	"repomedic/internal/config"
	gh "repomedic/internal/github"
	"strconv"
	"strings"

	"github.com/google/go-github/v66/github"
)

const defaultOrgDiscoveryRepoLimit = 1000

type RepositoryRef struct {
	Owner string
	Name  string
	ID    int64
	Repo  *github.Repository // Keep the full object if we have it
}

func ResolveRepos(ctx context.Context, client *gh.Client, cfg *config.Config) ([]RepositoryRef, error) {
	orgSel, userSel, err := normalizeTargetSelectors(cfg)
	if err != nil {
		return nil, err
	}

	// Enterprise scope (stub)
	if cfg.Targeting.Enterprise != "" {
		return nil, fmt.Errorf("enterprise scanning not yet implemented")
	}

	// Organization scope (optionally filtered by --repos selectors)
	if orgSel != "" {
		refs, err := listOrgRepoRefs(ctx, client, orgSel, computeRepoLimit(cfg))
		if err != nil {
			return nil, err
		}

		// Per UI spec: when used with org scope, --repos acts as an include-filter.
		refs, err = filterRefsByRepoSelectors(refs, cfg.Targeting.Repos)
		if err != nil {
			return nil, err
		}
		return dedupeRefs(refs), nil
	}

	// User scope (optionally filtered by --repos selectors)
	if userSel != "" {
		refs, err := listUserRepoRefs(ctx, client, userSel, computeRepoLimit(cfg))
		if err != nil {
			return nil, err
		}

		// In user scope, --repos acts as an include-filter (same semantics as org scope).
		refs, err = filterRefsByRepoSelectors(refs, cfg.Targeting.Repos)
		if err != nil {
			return nil, err
		}
		return dedupeRefs(refs), nil
	}

	// Explicit repos
	if len(cfg.Targeting.Repos) > 0 {
		refs, err := resolveExplicitRepoRefs(ctx, client, cfg.Targeting.Repos)
		if err != nil {
			return nil, err
		}
		return dedupeRefs(refs), nil
	}

	return nil, nil
}

func normalizeTargetSelectors(cfg *config.Config) (orgSel string, userSel string, err error) {
	orgSel = cfg.Targeting.Org
	userSel = cfg.Targeting.User

	if orgSel != "" {
		norm, nerr := normalizeAccountSelector(orgSel)
		if nerr != nil {
			return "", "", fmt.Errorf("invalid --org value: %w", nerr)
		}
		orgSel = norm
	}
	if userSel != "" {
		norm, nerr := normalizeAccountSelector(userSel)
		if nerr != nil {
			return "", "", fmt.Errorf("invalid --user value: %w", nerr)
		}
		userSel = norm
	}

	return orgSel, userSel, nil
}

func computeRepoLimit(cfg *config.Config) int {
	limit := defaultOrgDiscoveryRepoLimit
	if cfg.Targeting.MaxRepos > 0 {
		limit = cfg.Targeting.MaxRepos
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

func listOrgRepoRefs(ctx context.Context, client *gh.Client, org string, limit int) ([]RepositoryRef, error) {
	refs := make([]RepositoryRef, 0, min(limit, 100))

	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		repos, resp, err := client.Client.Repositories.ListByOrg(ctx, org, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list org repos: %w", err)
		}
		for _, repo := range repos {
			if len(refs) >= limit {
				break
			}
			refs = append(refs, RepositoryRef{
				Owner: repo.GetOwner().GetLogin(),
				Name:  repo.GetName(),
				ID:    repo.GetID(),
				Repo:  repo,
			})
		}
		if len(refs) >= limit {
			break
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return refs, nil
}

func listUserRepoRefs(ctx context.Context, client *gh.Client, user string, limit int) ([]RepositoryRef, error) {
	// If the requested user matches the authenticated token owner, use the
	// authenticated endpoint so private repos can be included.
	useAuthed := false
	if me, _, err := client.Client.Users.Get(ctx, ""); err == nil {
		if strings.EqualFold(me.GetLogin(), user) {
			useAuthed = true
		}
	}
	if useAuthed {
		return listAuthenticatedUserRepoRefs(ctx, client, limit)
	}
	return listPublicUserRepoRefs(ctx, client, user, limit)
}

func listAuthenticatedUserRepoRefs(ctx context.Context, client *gh.Client, limit int) ([]RepositoryRef, error) {
	refs := make([]RepositoryRef, 0, min(limit, 100))

	opts := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Visibility:  "all",
		Affiliation: "owner",
	}
	for {
		repos, resp, err := client.Client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list authenticated user repos: %w", err)
		}
		for _, repo := range repos {
			if len(refs) >= limit {
				break
			}
			refs = append(refs, RepositoryRef{
				Owner: repo.GetOwner().GetLogin(),
				Name:  repo.GetName(),
				ID:    repo.GetID(),
				Repo:  repo,
			})
		}
		if len(refs) >= limit {
			break
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return refs, nil
}

func listPublicUserRepoRefs(ctx context.Context, client *gh.Client, user string, limit int) ([]RepositoryRef, error) {
	refs := make([]RepositoryRef, 0, min(limit, 100))

	opts := &github.RepositoryListByUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Type:        "all",
	}
	for {
		repos, resp, err := client.Client.Repositories.ListByUser(ctx, user, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list user repos: %w", err)
		}
		for _, repo := range repos {
			if len(refs) >= limit {
				break
			}
			refs = append(refs, RepositoryRef{
				Owner: repo.GetOwner().GetLogin(),
				Name:  repo.GetName(),
				ID:    repo.GetID(),
				Repo:  repo,
			})
		}
		if len(refs) >= limit {
			break
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return refs, nil
}

func filterRefsByRepoSelectors(refs []RepositoryRef, selectors []string) ([]RepositoryRef, error) {
	if len(selectors) == 0 {
		return refs, nil
	}

	patterns := make([]string, 0, len(selectors))
	for _, sel := range selectors {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}
		if !hasGlobChars(sel) {
			norm, err := normalizeRepoSelector(sel)
			if err != nil {
				return nil, err
			}
			sel = norm
		}
		patterns = append(patterns, sel)
	}
	if len(patterns) == 0 {
		return refs, nil
	}

	filtered := make([]RepositoryRef, 0, len(refs))
	for _, r := range refs {
		fullName := r.Repo.GetFullName()
		repoName := r.Repo.GetName()
		matched := false
		for _, p := range patterns {
			if matchPattern(p, fullName, repoName) {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

func resolveExplicitRepoRefs(ctx context.Context, client *gh.Client, selectors []string) ([]RepositoryRef, error) {
	refs := make([]RepositoryRef, 0, len(selectors))

	for _, raw := range selectors {
		sel := strings.TrimSpace(raw)
		if sel == "" {
			continue
		}
		norm, err := normalizeRepoSelector(sel)
		if err != nil {
			return nil, err
		}
		sel = norm
		if hasGlobChars(sel) {
			return nil, fmt.Errorf("repo selector %q contains glob characters; use --org or --enterprise to enumerate candidates", sel)
		}
		owner, name, err := splitOwnerRepo(sel)
		if err != nil {
			return nil, err
		}
		repo, _, err := client.Client.Repositories.Get(ctx, owner, name)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve repo %s: %w", sel, err)
		}
		refs = append(refs, RepositoryRef{
			Owner: repo.GetOwner().GetLogin(),
			Name:  repo.GetName(),
			ID:    repo.GetID(),
			Repo:  repo,
		})
	}

	return refs, nil
}

func splitOwnerRepo(sel string) (owner string, name string, err error) {
	parts := strings.Split(sel, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
	}
	return parts[0], parts[1], nil
}

func hasGlobChars(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func normalizeRepoSelector(sel string) (string, error) {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return sel, nil
	}

	// Common URL forms:
	// - https://github.com/owner/repo
	// - https://github.com/owner/repo.git
	// - https://github.com/owner/repo/tree/main (we take owner/repo)
	// - github.com/owner/repo
	// - git@github.com:owner/repo.git

	if strings.HasPrefix(sel, "github.com/") || strings.HasPrefix(sel, "www.github.com/") {
		sel = "https://" + sel
	}

	if strings.HasPrefix(sel, "git@github.com:") {
		rest := strings.TrimPrefix(sel, "git@github.com:")
		rest = strings.Trim(rest, "/")
		parts := strings.Split(rest, "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
		}
		owner := parts[0]
		repo := strings.TrimSuffix(parts[1], ".git")
		if owner == "" || repo == "" {
			return "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
		}
		return owner + "/" + repo, nil
	}

	if strings.HasPrefix(sel, "http://") || strings.HasPrefix(sel, "https://") || strings.HasPrefix(sel, "git://") {
		u, err := url.Parse(sel)
		if err != nil {
			return "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
		}

		host := strings.ToLower(u.Hostname())
		if host == "www.github.com" {
			host = "github.com"
		}
		if host != "github.com" {
			// Only GitHub URLs are supported here; let non-GitHub URLs fail fast.
			return "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
		}

		path := strings.Trim(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
		}

		owner := parts[0]
		repo := strings.TrimSuffix(parts[1], ".git")
		if owner == "" || repo == "" {
			return "", fmt.Errorf("invalid repo selector %q; expected owner/name", sel)
		}
		return owner + "/" + repo, nil
	}

	return sel, nil
}

func normalizeAccountSelector(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "github.com/") || strings.HasPrefix(raw, "www.github.com/") {
		raw = "https://" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("%q", raw)
		}
		host := strings.ToLower(u.Hostname())
		if host == "www.github.com" {
			host = "github.com"
		}
		if host != "github.com" {
			return "", fmt.Errorf("%q", raw)
		}
		parts := strings.FieldsFunc(strings.Trim(u.Path, "/"), func(r rune) bool { return r == '/' })
		if len(parts) == 0 {
			return "", fmt.Errorf("%q", raw)
		}
		if parts[0] == "orgs" || parts[0] == "users" {
			if len(parts) < 2 {
				return "", fmt.Errorf("%q", raw)
			}
			return parts[1], nil
		}
		return parts[0], nil
	}
	if strings.Contains(raw, "/") {
		return "", fmt.Errorf("%q", raw)
	}
	return raw, nil
}

func dedupeRefs(in []RepositoryRef) []RepositoryRef {
	if len(in) <= 1 {
		return in
	}

	seen := make(map[string]struct{}, len(in))
	out := make([]RepositoryRef, 0, len(in))
	for _, r := range in {
		key := ""
		if r.ID != 0 {
			key = "id:" + strconv.FormatInt(r.ID, 10)
		} else if r.Repo != nil {
			key = "full:" + r.Repo.GetFullName()
		} else if r.Owner != "" && r.Name != "" {
			key = "full:" + r.Owner + "/" + r.Name
		}
		if key == "" {
			// If we can't construct a stable key, keep the entry.
			out = append(out, r)
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out
}
