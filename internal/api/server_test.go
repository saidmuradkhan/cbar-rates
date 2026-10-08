package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(NewCache(&stubFetcher{rates: fixtureRates(t)}, time.Hour))
}

func get(t *testing.T, srv http.Handler, url string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("%s: decode body: %v", url, err)
	}
	return rec.Code, body
}

func TestHealth(t *testing.T) {
	code, body := get(t, newTestServer(t), "/health")
	if code != http.StatusOK || body["status"] != "ok" {
		t.Errorf("got %d %v", code, body)
	}
}

func TestIndexListsEndpoints(t *testing.T) {
	code, body := get(t, newTestServer(t), "/api")
	if code != http.StatusOK || body["service"] != "cbar-rates" {
		t.Errorf("got %d %v", code, body)
	}
}

func TestConverterPage(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q", got)
	}
	for _, id := range []string{`id="converter"`, `id="chart"`} {
		if !strings.Contains(rec.Body.String(), id) {
			t.Errorf("page does not contain %s", id)
		}
	}
}

func TestListRates(t *testing.T) {
	code, body := get(t, newTestServer(t), "/rates")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if body["date"] != "2026-10-02" || body["base"] != "AZN" {
		t.Errorf("unexpected header fields: %v", body)
	}

	rates := body["rates"].([]any)
	if len(rates) != 42 {
		t.Fatalf("got %d rates, want 42", len(rates))
	}
	first := rates[0].(map[string]any)["code"].(string)
	last := rates[len(rates)-1].(map[string]any)["code"].(string)
	if first > last {
		t.Errorf("rates are not sorted: first %s, last %s", first, last)
	}
}

func TestGetRate(t *testing.T) {
	srv := newTestServer(t)

	code, body := get(t, srv, "/rates/usd")
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	rate := body["rate"].(map[string]any)
	if rate["code"] != "USD" || rate["value"] != 1.7 {
		t.Errorf("rate = %v", rate)
	}

	code, body = get(t, srv, "/rates/JPY")
	if got := body["rate"].(map[string]any)["per_unit"]; code != http.StatusOK || got != 0.010769 {
		t.Errorf("JPY per_unit = %v, want 0.010769", got)
	}

	code, _ = get(t, srv, "/rates/XYZ")
	if code != http.StatusNotFound {
		t.Errorf("unknown code status = %d, want 404", code)
	}
}

func TestConvert(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		url    string
		result float64
	}{
		{"/convert?from=USD&to=AZN&amount=1500", 2550},
		{"/convert?from=azn&to=usd&amount=170", 100},
		{"/convert?from=USD&to=USD&amount=42", 42},
		{"/convert?from=JPY&to=AZN&amount=10000", 107.69},
	}
	for _, tt := range tests {
		code, body := get(t, srv, tt.url)
		if code != http.StatusOK {
			t.Errorf("%s: status = %d, body %v", tt.url, code, body)
			continue
		}
		if body["result"] != tt.result {
			t.Errorf("%s: result = %v, want %v", tt.url, body["result"], tt.result)
		}
	}
}

func TestConvertBadInput(t *testing.T) {
	srv := newTestServer(t)

	urls := []string{
		"/convert?to=AZN&amount=1",
		"/convert?from=USD&to=AZN",
		"/convert?from=USD&to=AZN&amount=abc",
		"/convert?from=USD&to=AZN&amount=-5",
		"/convert?from=USD&to=AZN&amount=NaN",
		"/convert?from=XYZ&to=AZN&amount=1",
		"/convert?from=USD&to=XYZ&amount=1",
	}
	for _, url := range urls {
		code, body := get(t, srv, url)
		if code != http.StatusBadRequest || body["error"] == nil {
			t.Errorf("%s: got %d %v, want 400 with error", url, code, body)
		}
	}
}

func TestRatesUnavailable(t *testing.T) {
	srv := NewServer(NewCache(&stubFetcher{err: errors.New("cbar is down")}, time.Hour))

	code, body := get(t, srv, "/rates")
	if code != http.StatusBadGateway || body["error"] == nil {
		t.Errorf("got %d %v, want 502 with error", code, body)
	}
}

type datedFetcher struct {
	base *cbar.Rates

	mu        sync.Mutex
	requested []string
}

func (f *datedFetcher) Fetch(ctx context.Context, date time.Time) (*cbar.Rates, error) {
	f.mu.Lock()
	f.requested = append(f.requested, date.Format("2006-01-02"))
	f.mu.Unlock()
	return &cbar.Rates{Date: date, Rates: f.base.Rates}, nil
}

func newDatedServer(t *testing.T) (*Server, *datedFetcher) {
	t.Helper()
	f := &datedFetcher{base: fixtureRates(t)}
	clock := time.Date(2026, 10, 8, 12, 0, 0, 0, cbar.Baku)
	return NewServer(newTestCache(f, &clock)), f
}

func TestRatesForDate(t *testing.T) {
	srv, f := newDatedServer(t)

	code, body := get(t, srv, "/rates/USD?date=2026-09-15")
	if code != http.StatusOK || body["date"] != "2026-09-15" {
		t.Errorf("got %d %v", code, body)
	}
	code, body = get(t, srv, "/convert?from=USD&to=AZN&amount=10&date=2026-10-01")
	if code != http.StatusOK || body["date"] != "2026-10-01" {
		t.Errorf("got %d %v", code, body)
	}
	code, body = get(t, srv, "/rates")
	if code != http.StatusOK || body["date"] != "2026-10-08" {
		t.Errorf("without a date: got %d %v", code, body["date"])
	}
	if len(f.requested) != 3 {
		t.Errorf("requested %v", f.requested)
	}
}

func TestRatesForBadDate(t *testing.T) {
	srv, _ := newDatedServer(t)

	for _, url := range []string{
		"/rates?date=15.09.2026",
		"/rates?date=2026-13-01",
		"/rates?date=2026-10-09",
		"/convert?from=USD&to=AZN&amount=1&date=yesterday",
	} {
		code, body := get(t, srv, url)
		if code != http.StatusBadRequest || body["error"] == nil {
			t.Errorf("%s: got %d %v, want 400 with error", url, code, body)
		}
	}
}

func TestHistory(t *testing.T) {
	srv, f := newDatedServer(t)

	code, body := get(t, srv, "/history/usd?days=7")
	if code != http.StatusOK || body["code"] != "USD" {
		t.Fatalf("got %d %v", code, body)
	}
	points := body["points"].([]any)
	if len(points) != 7 {
		t.Fatalf("got %d points, want 7", len(points))
	}
	first := points[0].(map[string]any)
	last := points[6].(map[string]any)
	if first["date"] != "2026-10-02" || last["date"] != "2026-10-08" || last["per_unit"] != 1.7 {
		t.Errorf("first %v, last %v", first, last)
	}
	if len(f.requested) != 7 {
		t.Errorf("requested %d days, want 7", len(f.requested))
	}

	_, body = get(t, srv, "/history/USD")
	if got := len(body["points"].([]any)); got != 30 {
		t.Errorf("default history has %d points, want 30", got)
	}
}

func TestHistorySkipsRepeatedBulletins(t *testing.T) {
	friday := time.Date(2026, 10, 2, 0, 0, 0, 0, cbar.Baku)
	f := &stubFetcher{rates: &cbar.Rates{Date: friday, Rates: fixtureRates(t).Rates}}
	clock := time.Date(2026, 10, 4, 12, 0, 0, 0, cbar.Baku)
	srv := NewServer(newTestCache(f, &clock))

	_, body := get(t, srv, "/history/USD?days=3")
	if got := len(body["points"].([]any)); got != 1 {
		t.Errorf("got %d points, want 1 for a weekend", got)
	}
}

func TestHistoryErrors(t *testing.T) {
	srv, _ := newDatedServer(t)

	for _, url := range []string{"/history/USD?days=0", "/history/USD?days=91", "/history/USD?days=x"} {
		if code, _ := get(t, srv, url); code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", url, code)
		}
	}
	if code, _ := get(t, srv, "/history/XYZ?days=3"); code != http.StatusNotFound {
		t.Errorf("unknown code status = %d, want 404", code)
	}

	down := NewServer(NewCache(&stubFetcher{err: errors.New("cbar is down")}, time.Hour))
	if code, _ := get(t, down, "/history/USD?days=3"); code != http.StatusBadGateway {
		t.Errorf("CBAR down status = %d, want 502", code)
	}
}
