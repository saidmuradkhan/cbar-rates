package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
)

const (
	defaultHistoryDays = 30
	maxHistoryDays     = 90
	parallelFetches    = 6
)

type historyPoint struct {
	Date    string  `json:"date"`
	PerUnit float64 `json:"per_unit"`
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.PathValue("code"))
	days := defaultHistoryDays
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxHistoryDays {
			writeError(w, http.StatusBadRequest, "days must be between 1 and 90")
			return
		}
		days = n
	}

	bulletins := s.lastDays(r, days)

	points := []historyPoint{}
	failed := 0
	for _, rates := range bulletins {
		if rates == nil {
			failed++
			continue
		}
		rate, found := rates.Rates[code]
		if !found {
			continue
		}
		date := rates.Date.Format("2006-01-02")
		// CBAR answers weekends and holidays with the last working day's bulletin.
		if len(points) > 0 && points[len(points)-1].Date == date {
			continue
		}
		points = append(points, historyPoint{Date: date, PerUnit: round(rate.PerUnit(), 6)})
	}

	if len(points) == 0 {
		if failed == days {
			writeError(w, http.StatusBadGateway, "rates are unavailable right now")
		} else {
			writeError(w, http.StatusNotFound, "unknown currency: "+code)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":   code,
		"base":   "AZN",
		"points": points,
	})
}

// lastDays fetches the bulletins of the last n days, oldest first.
// A day that could not be fetched is nil.
func (s *Server) lastDays(r *http.Request, n int) []*cbar.Rates {
	today := s.cache.now()
	result := make([]*cbar.Rates, n)
	slots := make(chan struct{}, parallelFetches)

	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()

			rates, err := s.cache.Get(r.Context(), today.AddDate(0, 0, i-n+1))
			if err == nil {
				result[i] = rates
			}
		})
	}
	wg.Wait()
	return result
}
