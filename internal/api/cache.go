package api

import (
	"context"
	"sync"
	"time"

	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
)

type Fetcher interface {
	Fetch(ctx context.Context, date time.Time) (*cbar.Rates, error)
}

type cacheEntry struct {
	rates     *cbar.Rates
	fetchedAt time.Time
}

// Cache keeps fetched rates per day. Today's rates are refreshed after the TTL,
// a day that was already over when it was fetched never changes again.
type Cache struct {
	fetcher Fetcher
	ttl     time.Duration
	now     func() time.Time

	mu      sync.Mutex
	entries map[string]cacheEntry
}

func NewCache(fetcher Fetcher, ttl time.Duration) *Cache {
	return &Cache{
		fetcher: fetcher,
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]cacheEntry),
	}
}

func dayKey(t time.Time) string {
	return t.In(cbar.Baku).Format("2006-01-02")
}

func (c *Cache) Get(ctx context.Context, date time.Time) (*cbar.Rates, error) {
	key := dayKey(date)

	c.mu.Lock()
	entry, found := c.entries[key]
	c.mu.Unlock()

	if found && (dayKey(entry.fetchedAt) > key || c.now().Sub(entry.fetchedAt) < c.ttl) {
		return entry.rates, nil
	}

	rates, err := c.fetcher.Fetch(ctx, date)
	if err != nil {
		if found {
			return entry.rates, nil
		}
		return nil, err
	}

	c.mu.Lock()
	c.entries[key] = cacheEntry{rates: rates, fetchedAt: c.now()}
	c.mu.Unlock()
	return rates, nil
}

func (c *Cache) Today(ctx context.Context) (*cbar.Rates, error) {
	return c.Get(ctx, c.now())
}
