package fetcher

import (
	"golang.org/x/sync/singleflight"
)

type Group struct {
	g singleflight.Group
}

func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error, bool) {
	v, err, shared := g.g.Do(key, fn)
	return v, err, shared
}
