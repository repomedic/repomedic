package fetcher

import (
	"context"
	"fmt"
	"repomedic/internal/data"
	"sort"
	"sync"

	"github.com/google/go-github/v81/github"
)

type DataFetcher interface {
	Key() data.DependencyKey
	Scope() data.FetchScope
	Fetch(ctx context.Context, repo *github.Repository, params map[string]string, f *Fetcher) (any, error)
}

var (
	dataFetcherRegistry = make(map[data.DependencyKey]DataFetcher)
	dataFetcherMu       sync.RWMutex
)

func RegisterDataFetcher(df DataFetcher) {
	if df == nil {
		panic("data fetcher is nil")
	}
	k := df.Key()
	if k == "" {
		panic("data fetcher key is empty")
	}

	dataFetcherMu.Lock()
	defer dataFetcherMu.Unlock()
	if _, exists := dataFetcherRegistry[k]; exists {
		panic(fmt.Sprintf("data fetcher %s already registered", k))
	}
	dataFetcherRegistry[k] = df
}

func ResolveDataFetcher(key data.DependencyKey) (DataFetcher, bool) {
	dataFetcherMu.RLock()
	defer dataFetcherMu.RUnlock()
	df, ok := dataFetcherRegistry[key]
	return df, ok
}

func ListDataFetchers() []DataFetcher {
	dataFetcherMu.RLock()
	defer dataFetcherMu.RUnlock()

	all := make([]DataFetcher, 0, len(dataFetcherRegistry))
	for _, df := range dataFetcherRegistry {
		all = append(all, df)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Key() < all[j].Key()
	})
	return all
}
