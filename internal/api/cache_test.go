package api

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
)

type stubFetcher struct {
	rates *cbar.Rates
	err   error
	calls int
}

func (f *stubFetcher) Fetch(ctx context.Context, date time.Time) (*cbar.Rates, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.rates, nil
}

func fixtureRates(t *testing.T) *cbar.Rates {
	t.Helper()
	f, err := os.Open("../cbar/testdata/rates.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rates, err := cbar.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return rates
}

func newTestCache(f Fetcher, clock *time.Time) *Cache {
	c := NewCache(f, time.Hour)
	c.now = func() time.Time { return *clock }
	return c
}

func TestCacheReusesFreshEntry(t *testing.T) {
	clock := time.Date(2026, 10, 5, 10, 0, 0, 0, cbar.Baku)
	f := &stubFetcher{rates: fixtureRates(t)}
	c := newTestCache(f, &clock)

	for range 3 {
		if _, err := c.Today(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if f.calls != 1 {
		t.Errorf("fetch calls = %d, want 1", f.calls)
	}
}

func TestCacheRefetchesAfterTTL(t *testing.T) {
	clock := time.Date(2026, 10, 5, 10, 0, 0, 0, cbar.Baku)
	f := &stubFetcher{rates: fixtureRates(t)}
	c := newTestCache(f, &clock)

	c.Today(context.Background())
	clock = clock.Add(61 * time.Minute)
	c.Today(context.Background())

	if f.calls != 2 {
		t.Errorf("fetch calls = %d, want 2", f.calls)
	}
}

func TestCacheFallsBackToStaleEntry(t *testing.T) {
	clock := time.Date(2026, 10, 5, 10, 0, 0, 0, cbar.Baku)
	f := &stubFetcher{rates: fixtureRates(t)}
	c := newTestCache(f, &clock)

	c.Today(context.Background())
	clock = clock.Add(2 * time.Hour)
	f.err = errors.New("cbar is down")

	rates, err := c.Today(context.Background())
	if err != nil {
		t.Fatalf("expected stale rates, got error %v", err)
	}
	if rates.Rates["USD"].Value != 1.7 {
		t.Errorf("USD = %v, want 1.7", rates.Rates["USD"].Value)
	}
}

func TestCacheReturnsErrorWhenEmpty(t *testing.T) {
	clock := time.Now()
	c := newTestCache(&stubFetcher{err: errors.New("cbar is down")}, &clock)

	if _, err := c.Today(context.Background()); err == nil {
		t.Error("expected an error")
	}
}
