package cbar

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://www.cbar.az/currencies"
	dateLayout     = "02.01.2006"
	userAgent      = "cbar-rates/0.1 (+https://github.com/saidmuradkhan/cbar-rates)"
)

var Baku = time.FixedZone("AZT", 4*60*60)

type Rate struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Nominal float64 `json:"nominal"`
	Value   float64 `json:"value"`
}

// PerUnit is the price of one unit in AZN. CBAR quotes JPY, RUB and a few others per 100.
func (r Rate) PerUnit() float64 {
	return r.Value / r.Nominal
}

type Rates struct {
	Date  time.Time
	Rates map[string]Rate
}

type xmlValCurs struct {
	Date  string       `xml:"Date,attr"`
	Types []xmlValType `xml:"ValType"`
}

type xmlValType struct {
	Valutes []xmlValute `xml:"Valute"`
}

type xmlValute struct {
	Code    string `xml:"Code,attr"`
	Nominal string `xml:"Nominal"`
	Name    string `xml:"Name"`
	Value   string `xml:"Value"`
}

func Parse(r io.Reader) (*Rates, error) {
	var doc xmlValCurs
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode xml: %w", err)
	}

	date, err := time.ParseInLocation(dateLayout, doc.Date, Baku)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", doc.Date, err)
	}

	result := &Rates{Date: date, Rates: make(map[string]Rate)}
	for _, t := range doc.Types {
		for _, v := range t.Valutes {
			rate, err := v.toRate()
			if err != nil {
				return nil, err
			}
			result.Rates[rate.Code] = rate
		}
	}
	if len(result.Rates) == 0 {
		return nil, fmt.Errorf("no rates in document")
	}
	return result, nil
}

func (v xmlValute) toRate() (Rate, error) {
	nominalField := strings.Fields(v.Nominal)
	if len(nominalField) == 0 {
		return Rate{}, fmt.Errorf("%s: empty nominal", v.Code)
	}
	nominal, err := strconv.ParseFloat(nominalField[0], 64)
	if err != nil || nominal <= 0 {
		return Rate{}, fmt.Errorf("%s: bad nominal %q", v.Code, v.Nominal)
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(v.Value), 64)
	if err != nil {
		return Rate{}, fmt.Errorf("%s: bad value %q", v.Code, v.Value)
	}
	return Rate{
		Code:    strings.ToUpper(strings.TrimSpace(v.Code)),
		Name:    strings.TrimSpace(v.Name),
		Nominal: nominal,
		Value:   value,
	}, nil
}

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Fetch(ctx context.Context, date time.Time) (*Rates, error) {
	url := fmt.Sprintf("%s/%s.xml", c.BaseURL, date.In(Baku).Format(dateLayout))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %d", url, resp.StatusCode)
	}
	return Parse(resp.Body)
}
