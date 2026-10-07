package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	notFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})

	rec := httptest.NewRecorder()
	LogRequests(logger, notFound).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/rates/XYZ", nil))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, buf.String())
	}
	if line["msg"] != "request" || line["method"] != "GET" || line["path"] != "/rates/XYZ" {
		t.Errorf("unexpected log line: %v", line)
	}
	if line["status"] != float64(http.StatusNotFound) {
		t.Errorf("status = %v, want 404", line["status"])
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("response status = %d", rec.Code)
	}
}

func TestLogRequestsDefaultsToOK(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	LogRequests(logger, ok).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var line map[string]any
	json.Unmarshal(buf.Bytes(), &line)
	if line["status"] != float64(http.StatusOK) {
		t.Errorf("status = %v, want 200", line["status"])
	}
}
