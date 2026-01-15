package providers

import (
	"testing"

	"repomedic/internal/data/models"

	"github.com/google/go-github/v81/github"
)

func TestSampleReposForConvention(t *testing.T) {
	tests := []struct {
		name     string
		repos    []*github.Repository
		n        int
		wantLen  int
		wantName string // Expected first repo name (descending order).
	}{
		{
			name:    "empty repos",
			repos:   []*github.Repository{},
			n:       10,
			wantLen: 0,
		},
		{
			name: "fewer than n repos",
			repos: []*github.Repository{
				{FullName: github.Ptr("org/a")},
				{FullName: github.Ptr("org/b")},
			},
			n:        10,
			wantLen:  2,
			wantName: "org/b", // 'b' > 'a' in descending order
		},
		{
			name: "exactly n repos",
			repos: []*github.Repository{
				{FullName: github.Ptr("org/a")},
				{FullName: github.Ptr("org/b")},
				{FullName: github.Ptr("org/c")},
			},
			n:        3,
			wantLen:  3,
			wantName: "org/c",
		},
		{
			name: "more than n repos",
			repos: []*github.Repository{
				{FullName: github.Ptr("org/a")},
				{FullName: github.Ptr("org/b")},
				{FullName: github.Ptr("org/c")},
				{FullName: github.Ptr("org/d")},
			},
			n:        2,
			wantLen:  2,
			wantName: "org/d",
		},
		{
			name: "case insensitive sorting",
			repos: []*github.Repository{
				{FullName: github.Ptr("org/Abc")},
				{FullName: github.Ptr("org/xyz")},
				{FullName: github.Ptr("org/MNO")},
			},
			n:        3,
			wantLen:  3,
			wantName: "org/xyz", // 'xyz' > 'mno' > 'abc' when lowercased
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sampleReposForConvention(tt.repos, tt.n)
			if len(got) != tt.wantLen {
				t.Errorf("sampleReposForConvention() returned %d repos, want %d", len(got), tt.wantLen)
			}
			if tt.wantName != "" && len(got) > 0 {
				if got[0].GetFullName() != tt.wantName {
					t.Errorf("first repo = %q, want %q", got[0].GetFullName(), tt.wantName)
				}
			}
		})
	}
}

func TestMaskFromRepoBools(t *testing.T) {
	tests := []struct {
		name string
		repo *github.Repository
		want models.MergeMethodMask
	}{
		{
			name: "all methods enabled",
			repo: &github.Repository{
				AllowMergeCommit: github.Ptr(true),
				AllowSquashMerge: github.Ptr(true),
				AllowRebaseMerge: github.Ptr(true),
			},
			want: models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase,
		},
		{
			name: "only squash",
			repo: &github.Repository{
				AllowMergeCommit: github.Ptr(false),
				AllowSquashMerge: github.Ptr(true),
				AllowRebaseMerge: github.Ptr(false),
			},
			want: models.MergeMethodSquash,
		},
		{
			name: "merge and rebase",
			repo: &github.Repository{
				AllowMergeCommit: github.Ptr(true),
				AllowSquashMerge: github.Ptr(false),
				AllowRebaseMerge: github.Ptr(true),
			},
			want: models.MergeMethodMerge | models.MergeMethodRebase,
		},
		{
			name: "nil booleans default to false",
			repo: &github.Repository{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskFromRepoBools(tt.repo)
			if got != tt.want {
				t.Errorf("maskFromRepoBools() = %v (%s), want %v (%s)", got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

func TestSelectConventionWinner(t *testing.T) {
	allMethods := models.MergeMethodMerge | models.MergeMethodSquash | models.MergeMethodRebase
	squashOnly := models.MergeMethodSquash
	mergeOnly := models.MergeMethodMerge
	rebaseOnly := models.MergeMethodRebase

	tests := []struct {
		name      string
		counts    map[models.MergeMethodMask]int
		wantMask  models.MergeMethodMask
		wantConfl bool
	}{
		{
			name:      "empty counts is conflict",
			counts:    map[models.MergeMethodMask]int{},
			wantMask:  0,
			wantConfl: true,
		},
		{
			name:      "single winner",
			counts:    map[models.MergeMethodMask]int{squashOnly: 5},
			wantMask:  squashOnly,
			wantConfl: false,
		},
		{
			name:      "clear winner by count",
			counts:    map[models.MergeMethodMask]int{squashOnly: 5, allMethods: 2},
			wantMask:  squashOnly,
			wantConfl: false,
		},
		{
			name:      "tie broken by smaller size",
			counts:    map[models.MergeMethodMask]int{squashOnly: 3, allMethods: 3},
			wantMask:  squashOnly,
			wantConfl: false,
		},
		{
			name:      "conflict when same size and not subsets",
			counts:    map[models.MergeMethodMask]int{squashOnly: 3, mergeOnly: 3, rebaseOnly: 3},
			wantMask:  0,
			wantConfl: true, // All size 1, none is subset of another.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMask, gotConfl := selectConventionWinner(tt.counts)
			if gotConfl != tt.wantConfl {
				t.Errorf("selectConventionWinner() conflict = %v, want %v", gotConfl, tt.wantConfl)
			}
			if !tt.wantConfl && gotMask != tt.wantMask {
				t.Errorf("selectConventionWinner() mask = %v (%s), want %v (%s)",
					gotMask, gotMask.String(), tt.wantMask, tt.wantMask.String())
			}
		})
	}
}

func TestSelectConventionWinner_SubsetWins(t *testing.T) {
	// squashOnly is subset of squashRebase
	squashOnly := models.MergeMethodSquash
	squashRebase := models.MergeMethodSquash | models.MergeMethodRebase

	counts := map[models.MergeMethodMask]int{
		squashOnly:   3,
		squashRebase: 3,
	}

	gotMask, gotConfl := selectConventionWinner(counts)
	if gotConfl {
		t.Errorf("expected no conflict when subset exists")
	}
	if gotMask != squashOnly {
		t.Errorf("selectConventionWinner() mask = %v (%s), want %v (%s)",
			gotMask, gotMask.String(), squashOnly, squashOnly.String())
	}
}
