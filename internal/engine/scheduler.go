package engine

import (
	"context"
	"errors"
	"fmt"
	"repomedic/internal/data"
	"repomedic/internal/fetcher"
	"sort"
	"sync"
)

type Scheduler struct {
	fetcher     *fetcher.Fetcher
	concurrency int
}

func NewScheduler(f *fetcher.Fetcher, concurrency int) (*Scheduler, error) {
	if f == nil {
		return nil, errors.New("fetcher is nil")
	}
	if concurrency <= 0 {
		return nil, fmt.Errorf("concurrency must be >= 1, got %d", concurrency)
	}
	return &Scheduler{fetcher: f, concurrency: concurrency}, nil
}

// Execute streams per-repo dependency fetch completion results.
//
// Channel semantics:
//   - In the normal (non-canceled) case, exactly one RepoExecutionResult is sent per repo.
//   - On context cancellation, the scheduler stops promptly; it may emit fewer than N results.
//   - The results channel and error channel are both closed reliably.
//   - The error channel is used for fatal errors / cancellation signals; per-dependency
//     fetch failures are recorded on RepoExecutionResult.DepErrs.
func (s *Scheduler) Execute(ctx context.Context, plan *ScanPlan) (<-chan RepoExecutionResult, <-chan error) {
	resultsCh := make(chan RepoExecutionResult)
	errCh := make(chan error, 1)

	go func() {
		defer close(resultsCh)
		defer close(errCh)

		trySendErr := func(err error) {
			if err == nil {
				return
			}
			select {
			case errCh <- err:
			default:
			}
		}

		if ctx == nil {
			trySendErr(errors.New("context is nil"))
			return
		}
		if plan == nil {
			trySendErr(errors.New("scan plan is nil"))
			return
		}
		if plan.RepoPlans == nil {
			trySendErr(errors.New("scan plan is not initialized (RepoPlans is nil); use NewScanPlan"))
			return
		}
		if s == nil {
			trySendErr(errors.New("scheduler is nil"))
			return
		}
		if s.fetcher == nil {
			trySendErr(errors.New("scheduler fetcher is nil"))
			return
		}
		if s.concurrency <= 0 {
			trySendErr(fmt.Errorf("scheduler concurrency must be >= 1, got %d", s.concurrency))
			return
		}

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Limit active repos (favor repo completion).
		sem := make(chan struct{}, s.concurrency)
		var wg sync.WaitGroup

		repoIDs := make([]int64, 0, len(plan.RepoPlans))
		for id := range plan.RepoPlans {
			repoIDs = append(repoIDs, id)
		}
		sort.Slice(repoIDs, func(i, j int) bool { return repoIDs[i] < repoIDs[j] })

		var fatalErr error

	scheduleLoop:
		for _, repoID := range repoIDs {
			if runCtx.Err() != nil {
				break
			}
			rp := plan.RepoPlans[repoID]
			if rp == nil {
				fatalErr = errors.New("nil repo plan")
				cancel()
				break
			}

			select {
			case sem <- struct{}{}:
				// acquired
			case <-runCtx.Done():
				break scheduleLoop
			}

			wg.Add(1)
			go func(rp *RepoPlan) {
				defer wg.Done()
				defer func() { <-sem }()

				dataMap := make(map[data.DependencyKey]any)
				depErrs := make(map[data.DependencyKey]error)

				deps := rp.SortedDependencies()
				for _, key := range deps {
					if runCtx.Err() != nil {
						return
					}
					req := rp.Dependencies[key]
					val, err := s.fetcher.Fetch(runCtx, rp.Repo.Repo, req.Key, req.Params)
					if err != nil {
						depErrs[req.Key] = err
						continue
					}
					dataMap[req.Key] = val
				}

				if runCtx.Err() != nil {
					return
				}

				res := RepoExecutionResult{
					RepoID:  rp.Repo.ID,
					Data:    data.NewMapDataContext(dataMap),
					DepErrs: depErrs,
				}
				select {
				case resultsCh <- res:
				case <-runCtx.Done():
					return
				}
			}(rp)
		}

		wg.Wait()
		if fatalErr != nil {
			trySendErr(fatalErr)
			return
		}
		trySendErr(ctx.Err())
	}()

	return resultsCh, errCh
}
