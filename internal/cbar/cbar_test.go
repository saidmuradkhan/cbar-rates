package cbar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func openFixture(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open("testdata/rates.xml")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestParse(t *testing.T) {
	rates, err := Parse(openFixture(t))
	if err != nil {
		t.Fatal(err)
	}

	if got := rates.Date.Format("2006-01-02"); got != "2026-10-02" {
		t.Errorf("date = %s, want 2026-10-02", got)
	}
	if len(rates.Rates) != 42 {
		t.Errorf("got %d rates, want 42", len(rates.Rates))
	}

	tests := []struct {
		code    string
		nominal float64
		value   float64
	}{
		{"USD", 1, 1.7},
		{"XAU", 1, 7114.3555},
		{"JPY", 100, 1.0769},
	}
	for _, tt := range tests {
		r, ok := rates.Rates[tt.code]
		if !ok {
			t.Errorf("%s missing", tt.code)
			continue
		}
		if r.Nominal != tt.nominal || r.Value != tt.value {
			t.Errorf("%s = %+v, want nominal %v value %v", tt.code, r, tt.nominal, tt.value)
		}
	}
}

func TestPerUnit(t *testing.T) {
	r := Rate{Code: "JPY", Nominal: 100, Value: 1.15}
	if got := r.PerUnit(); got != 0.0115 {
		t.Errorf("PerUnit = %v, want 0.0115", got)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"not xml":      "hello",
		"bad date":     `<ValCurs Date="yesterday"><ValType><Valute Code="USD"><Nominal>1</Nominal><Value>1.7</Value></Valute></ValType></ValCurs>`,
		"bad value":    `<ValCurs Date="02.10.2026"><ValType><Valute Code="USD"><Nominal>1</Nominal><Value>abc</Value></Valute></ValType></ValCurs>`,
		"zero nominal": `<ValCurs Date="02.10.2026"><ValType><Valute Code="USD"><Nominal>0</Nominal><Value>1.7</Value></Valute></ValType></ValCurs>`,
		"empty":        `<ValCurs Date="02.10.2026"></ValCurs>`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(body)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestClientFetch(t *testing.T) {
	var gotPath, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUA = r.UserAgent()
		http.ServeFile(w, r, "testdata/rates.xml")
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL, HTTP: ts.Client()}
	date := time.Date(2026, 10, 3, 0, 0, 0, 0, Baku)
	rates, err := c.Fetch(context.Background(), date)
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/03.10.2026.xml" {
		t.Errorf("path = %s, want /03.10.2026.xml", gotPath)
	}
	if !strings.HasPrefix(gotUA, "cbar-rates/") {
		t.Errorf("user agent = %q", gotUA)
	}
	if rates.Rates["USD"].Value != 1.7 {
		t.Errorf("USD = %v, want 1.7", rates.Rates["USD"].Value)
	}
}

func TestClientFetchBadStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL, HTTP: ts.Client()}
	if _, err := c.Fetch(context.Background(), time.Now()); err == nil {
		t.Error("expected an error for 503")
	}
}
