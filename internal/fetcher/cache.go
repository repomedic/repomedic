package fetcher

import "sync"

type Cache struct {
	data sync.Map
}

func NewCache() *Cache {
	return &Cache{}
}

func (c *Cache) Get(key string) (any, bool) {
	return c.data.Load(key)
}

func (c *Cache) Set(key string, value any) {
	c.data.Store(key, value)
}
