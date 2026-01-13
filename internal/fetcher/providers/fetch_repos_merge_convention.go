package providers

import (
	"context"
	"sort"
	"strings"

	"repomedic/internal/data"
	"repomedic/internal/data/models"
	"repomedic/internal/fetcher"

	"github.com/google/go-github/v66/github"
)

// Default sample size for convention derivation.
// This keeps API calls bounded while still providing a reasonable sample.
const defaultConventionSampleSize = 10

// reposMergeConventionFetcher derives a merge-method baseline from convention
// by sampling repos and finding the most common merge method configuration.
//
// Algorithm:
//  1. Sort repos deterministically.
//  2. Sample top N repos.
//  3. For each repo, get merge method settings (prefer existing metadata, else fetch).
//  4. Find the most common MergeMethodMask.
//  5. Apply tie-breaking: prefer smaller masks, then subset relationships.
type reposMergeConventionFetcher struct{}

func (r *reposMergeConventionFetcher) Key() data.DependencyKey {
	return data.DepReposMergeConvention
}

func (r *reposMergeConventionFetcher) Scope() data.FetchScope {
	return data.ScopeOrg
}

func (r *reposMergeConventionFetcher) Fetch(ctx context.Context, repo *github.Repository, _ map[string]string, f *fetcher.Fetcher) (any, error) {
	// Get scanned repos.
	scannedResult, err := f.Fetch(ctx, repo, data.DepReposScanned, nil)
	if err != nil {
		return nil, err
	}
	scannedRepos, ok := scannedResult.([]*github.Repository)
	if !ok {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceConvention,
			Evidence: []string{"invalid scanned repos type"},
		}, nil
	}

	if len(scannedRepos) == 0 {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceConvention,
			Evidence: []string{"no scanned repos available"},
		}, nil
	}

	// Sample repos deterministically.
	sampleRepos := sampleReposForConvention(scannedRepos, defaultConventionSampleSize)

	// Collect merge method masks from samples.
	maskCounts := make(map[models.MergeMethodMask]int)
	var evidence []string

	for _, sampleRepo := range sampleRepos {
		mask := getMergeMethodMaskFromRepo(ctx, sampleRepo, f)
		if mask == 0 {
			// Skip repos with no merge methods enabled (likely error or archived).
			continue
		}
		maskCounts[mask]++
		evidence = append(evidence, sampleRepo.GetFullName()+": "+mask.String())
	}

	if len(maskCounts) == 0 {
		return &models.MergeBaseline{
			State:    models.BaselineStateNone,
			Source:   models.BaselineSourceConvention,
			Evidence: []string{"no valid merge configurations found in sample"},
		}, nil
	}

	// Find winner with tie-breaking.
	winner, isConflict := selectConventionWinner(maskCounts)

	if isConflict {
		return &models.MergeBaseline{
			State:    models.BaselineStateConflict,
			Source:   models.BaselineSourceConvention,
			Allowed:  0,
			Evidence: evidence,
		}, nil
	}

	return &models.MergeBaseline{
		State:    models.BaselineStateSet,
		Source:   models.BaselineSourceConvention,
		Allowed:  winner,
		Evidence: evidence,
	}, nil
}

// sampleReposForConvention returns a deterministic sample of repos.
// Sorts by lowercase owner/name descending and takes top N.
func sampleReposForConvention(repos []*github.Repository, n int) []*github.Repository {
	// Make a copy to avoid mutating the original slice.
	sorted := make([]*github.Repository, len(repos))
	copy(sorted, repos)

	// Sort by lowercase full name descending for determinism.
	sort.Slice(sorted, func(i, j int) bool {
		iName := strings.ToLower(sorted[i].GetFullName())
		jName := strings.ToLower(sorted[j].GetFullName())
		return iName > jName // Descending order.
	})

	if len(sorted) > n {
		sorted = sorted[:n]
	}

	return sorted
}

// getMergeMethodMaskFromRepo extracts the merge method mask from repo metadata.
// If the repo lacks the necessary fields, it fetches metadata via the fetcher.
func getMergeMethodMaskFromRepo(ctx context.Context, repo *github.Repository, f *fetcher.Fetcher) models.MergeMethodMask {
	// Check if repo already has the merge method booleans.
	if repo.AllowMergeCommit != nil || repo.AllowSquashMerge != nil || repo.AllowRebaseMerge != nil {
		return maskFromRepoBools(repo)
	}

	// Fetch full metadata.
	metaResult, err := f.Fetch(ctx, repo, data.DepRepoMetadata, nil)
	if err != nil {
		return 0
	}

	fullRepo, ok := metaResult.(*github.Repository)
	if !ok || fullRepo == nil {
		return 0
	}

	return maskFromRepoBools(fullRepo)
}

// maskFromRepoBools converts repo boolean settings to a MergeMethodMask.
func maskFromRepoBools(repo *github.Repository) models.MergeMethodMask {
	var mask models.MergeMethodMask

	if repo.GetAllowMergeCommit() {
		mask |= models.MergeMethodMerge
	}
	if repo.GetAllowSquashMerge() {
		mask |= models.MergeMethodSquash
	}
	if repo.GetAllowRebaseMerge() {
		mask |= models.MergeMethodRebase
	}

	return mask
}

// selectConventionWinner finds the winning mask from frequency counts.
// Tie-breaking rules:
//  1. Highest frequency wins.
//  2. Among ties: prefer mask with smallest Size().
//  3. If still tied: prefer mask that is a subset of all other tied masks.
//  4. If no clear winner: conflict.
func selectConventionWinner(counts map[models.MergeMethodMask]int) (models.MergeMethodMask, bool) {
	if len(counts) == 0 {
		return 0, true
	}

	// Find max count.
	maxCount := 0
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}

	// Collect all masks with max count.
	var winners []models.MergeMethodMask
	for mask, count := range counts {
		if count == maxCount {
			winners = append(winners, mask)
		}
	}

	// Sort winners for determinism.
	sort.Slice(winners, func(i, j int) bool {
		return winners[i] < winners[j]
	})

	if len(winners) == 1 {
		return winners[0], false
	}

	// Tie-break 1: prefer smallest Size().
	minSize := winners[0].Size()
	var minSizeWinners []models.MergeMethodMask
	for _, w := range winners {
		size := w.Size()
		if size < minSize {
			minSize = size
			minSizeWinners = []models.MergeMethodMask{w}
		} else if size == minSize {
			minSizeWinners = append(minSizeWinners, w)
		}
	}

	if len(minSizeWinners) == 1 {
		return minSizeWinners[0], false
	}

	// Tie-break 2: check for unique subset relationship.
	// If one mask is a subset of all others, it wins.
	for _, candidate := range minSizeWinners {
		isSubsetOfAll := true
		for _, other := range minSizeWinners {
			if candidate == other {
				continue
			}
			// candidate is subset of other if candidate & other == candidate
			if candidate.Intersect(other) != candidate {
				isSubsetOfAll = false
				break
			}
		}
		if isSubsetOfAll {
			return candidate, false
		}
	}

	// No clear winner - conflict.
	return 0, true
}

func init() {
	fetcher.RegisterDataFetcher(&reposMergeConventionFetcher{})
}
