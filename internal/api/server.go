package api

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/saidmuradkhan/cbar-rates/internal/cbar"
)

//go:embed page.html
var converterPage []byte

type Server struct {
	mux   *http.ServeMux
	cache *Cache
}

func NewServer(cache *Cache) *Server {
	s := &Server{mux: http.NewServeMux(), cache: cache}
	s.mux.HandleFunc("GET /{$}", s.page)
	s.mux.HandleFunc("GET /api", s.index)
	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("GET /rates", s.listRates)
	s.mux.HandleFunc("GET /rates/{code}", s.getRate)
	s.mux.HandleFunc("GET /convert", s.convert)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type rateJSON struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Nominal float64 `json:"nominal"`
	Value   float64 `json:"value"`
	PerUnit float64 `json:"per_unit"`
}

func toRateJSON(r cbar.Rate) rateJSON {
	return rateJSON{
		Code:    r.Code,
		Name:    r.Name,
		Nominal: r.Nominal,
		Value:   r.Value,
		PerUnit: round(r.PerUnit(), 6),
	}
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(converterPage)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "cbar-rates",
		"endpoints": []string{
			"/",
			"/health",
			"/rates",
			"/rates/{code}",
			"/convert?from=USD&to=AZN&amount=100",
		},
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listRates(w http.ResponseWriter, r *http.Request) {
	rates, ok := s.todayRates(w, r)
	if !ok {
		return
	}

	list := make([]rateJSON, 0, len(rates.Rates))
	for _, rate := range rates.Rates {
		list = append(list, toRateJSON(rate))
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Code < list[j].Code })

	writeJSON(w, http.StatusOK, map[string]any{
		"date":  rates.Date.Format("2006-01-02"),
		"base":  "AZN",
		"rates": list,
	})
}

func (s *Server) getRate(w http.ResponseWriter, r *http.Request) {
	rates, ok := s.todayRates(w, r)
	if !ok {
		return
	}

	code := strings.ToUpper(r.PathValue("code"))
	rate, found := rates.Rates[code]
	if !found {
		writeError(w, http.StatusNotFound, "unknown currency: "+code)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"date": rates.Date.Format("2006-01-02"),
		"base": "AZN",
		"rate": toRateJSON(rate),
	})
}

func (s *Server) convert(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := strings.ToUpper(q.Get("from"))
	to := strings.ToUpper(q.Get("to"))
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "from and to are required")
		return
	}
	amount, err := strconv.ParseFloat(q.Get("amount"), 64)
	if err != nil || amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		writeError(w, http.StatusBadRequest, "amount must be a non-negative number")
		return
	}

	rates, ok := s.todayRates(w, r)
	if !ok {
		return
	}

	fromAZN, found := perUnitAZN(rates, from)
	if !found {
		writeError(w, http.StatusBadRequest, "unknown currency: "+from)
		return
	}
	toAZN, found := perUnitAZN(rates, to)
	if !found {
		writeError(w, http.StatusBadRequest, "unknown currency: "+to)
		return
	}

	rate := fromAZN / toAZN
	writeJSON(w, http.StatusOK, map[string]any{
		"date":   rates.Date.Format("2006-01-02"),
		"from":   from,
		"to":     to,
		"amount": amount,
		"rate":   round(rate, 6),
		"result": round(amount*rate, 2),
	})
}

func perUnitAZN(rates *cbar.Rates, code string) (float64, bool) {
	if code == "AZN" {
		return 1, true
	}
	rate, found := rates.Rates[code]
	return rate.PerUnit(), found
}

func (s *Server) todayRates(w http.ResponseWriter, r *http.Request) (*cbar.Rates, bool) {
	rates, err := s.cache.Today(r.Context())
	if err != nil {
		slog.Error("fetch rates failed", "err", err)
		writeError(w, http.StatusBadGateway, "rates are unavailable right now")
		return nil, false
	}
	return rates, true
}

func round(x float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(x*p) / p
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
