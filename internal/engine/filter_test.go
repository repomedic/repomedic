package engine

import (
	"repomedic/internal/config"
	"slices"
	"testing"

	"github.com/google/go-github/v81/github"
)

func TestFilterRepos(t *testing.T) {
	repos := []RepositoryRef{
		{Repo: &github.Repository{Name: github.Ptr("repo1"), FullName: github.Ptr("acme/repo1"), Private: github.Ptr(false), Archived: github.Ptr(false), Fork: github.Ptr(false), Topics: []string{"go"}}},
		{Repo: &github.Repository{Name: github.Ptr("repo2"), FullName: github.Ptr("acme/repo2"), Private: github.Ptr(true), Archived: github.Ptr(false), Fork: github.Ptr(false), Topics: []string{"js"}}},
		{Repo: &github.Repository{Name: github.Ptr("repo3"), FullName: github.Ptr("acme/repo3"), Private: github.Ptr(false), Archived: github.Ptr(true), Fork: github.Ptr(false)}},
		{Repo: &github.Repository{Name: github.Ptr("repo4"), FullName: github.Ptr("acme/repo4"), Private: github.Ptr(false), Archived: github.Ptr(false), Fork: github.Ptr(true)}},
		{Repo: &github.Repository{Name: github.Ptr("repo5"), FullName: github.Ptr("acme/repo5"), Private: github.Ptr(true), Visibility: github.Ptr("internal"), Archived: github.Ptr(false), Fork: github.Ptr(false)}},
	}

	tests := []struct {
		name     string
		cfg      func() *config.Config
		expected []string
	}{
		{
			name: "All defaults (exclude archived/forks)",
			cfg: func() *config.Config {
				c := config.New()
				return c
			},
			expected: []string{"repo1", "repo2", "repo5"},
		},
		{
			name: "Include archived",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Archived = "include"
				return c
			},
			expected: []string{"repo1", "repo2", "repo3", "repo5"},
		},
		{
			name: "Only public",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Visibility = "public"
				return c
			},
			expected: []string{"repo1"},
		},
		{
			name: "Only internal",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Visibility = "internal"
				return c
			},
			expected: []string{"repo5"},
		},
		{
			name: "Topic filter",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Topic = []string{"go"}
				return c
			},
			expected: []string{"repo1"},
		},
		{
			name: "Include pattern",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Include = []string{"*repo2"}
				return c
			},
			expected: []string{"repo2"},
		},
		{
			name: "Owner-qualified include pattern",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Include = []string{"acme/repo2"}
				return c
			},
			expected: []string{"repo2"},
		},
		{
			name: "Exclude pattern",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Exclude = []string{"*repo1"}
				return c
			},
			expected: []string{"repo2", "repo5"},
		},
		{
			name: "Include then exclude (exclude wins)",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Include = []string{"repo*"}
				c.Targeting.Exclude = []string{"*repo1"}
				return c
			},
			expected: []string{"repo2", "repo5"},
		},
		{
			name: "Max repos",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.MaxRepos = 1
				return c
			},
			expected: []string{"repo1"},
		},
		{
			name: "Invalid include pattern matches nothing",
			cfg: func() *config.Config {
				c := config.New()
				c.Targeting.Include = []string{"["}
				return c
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterRepos(repos, tt.cfg())
			var got []string
			for _, r := range filtered {
				got = append(got, r.Repo.GetName())
			}
			if !slices.Equal(got, tt.expected) {
				t.Fatalf("Expected repos %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestFilterRepos_NilConfigPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic, got none")
		}
	}()

	_ = FilterRepos(nil, nil)
}
