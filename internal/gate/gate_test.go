package gate

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestHandler() (*Gate, http.Handler) {
	g := New("reviewer", "letmein", "test-secret")
	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret page"))
	})
	return g, g.Wrap(app)
}

func do(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func loginRequest(password, next string) *http.Request {
	form := url.Values{"username": {"reviewer"}, "password": {password}, "next": {next}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestNilGateIsOpen(t *testing.T) {
	var g *Gate
	h := g.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if rec := do(h, httptest.NewRequest(http.MethodGet, "/rates", nil)); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("PREVIEW_USER", "")
	t.Setenv("PREVIEW_PASSWORD", "")
	if FromEnv() != nil {
		t.Error("gate should be off without credentials")
	}

	t.Setenv("PREVIEW_USER", "a")
	t.Setenv("PREVIEW_PASSWORD", "b")
	t.Setenv("PREVIEW_SECRET", "")
	if g := FromEnv(); g == nil || g.Secret != "b" {
		t.Errorf("got %+v, want secret to fall back to the password", g)
	}
}

func TestBlocksWithoutLogin(t *testing.T) {
	_, h := newTestHandler()

	rec := do(h, httptest.NewRequest(http.MethodGet, "/rates", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("api status = %d, want 401", rec.Code)
	}

	r := httptest.NewRequest(http.MethodGet, "/rates", nil)
	r.Header.Set("Accept", "text/html")
	rec = do(h, r)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login?next=%2Frates" {
		t.Errorf("browser got %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHealthIsOpen(t *testing.T) {
	_, h := newTestHandler()
	if rec := do(h, httptest.NewRequest(http.MethodGet, "/health", nil)); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestLoginFlow(t *testing.T) {
	_, h := newTestHandler()

	if rec := do(h, httptest.NewRequest(http.MethodGet, "/login", nil)); rec.Code != http.StatusOK {
		t.Fatalf("login page status = %d", rec.Code)
	}

	rec := do(h, loginRequest("letmein", "/rates"))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/rates" {
		t.Fatalf("login got %d %s", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected cookies: %v", cookies)
	}

	r := httptest.NewRequest(http.MethodGet, "/rates", nil)
	r.AddCookie(cookies[0])
	if rec := do(h, r); rec.Code != http.StatusOK || rec.Body.String() != "secret page" {
		t.Errorf("with cookie got %d %q", rec.Code, rec.Body.String())
	}
}

func TestWrongPassword(t *testing.T) {
	_, h := newTestHandler()
	rec := do(h, loginRequest("nope", "/"))
	if rec.Code != http.StatusUnauthorized || len(rec.Result().Cookies()) != 0 {
		t.Errorf("got %d with cookies %v", rec.Code, rec.Result().Cookies())
	}
	if !strings.Contains(rec.Body.String(), "Wrong username or password") {
		t.Error("error message missing")
	}
}

func TestLoginIgnoresExternalNext(t *testing.T) {
	_, h := newTestHandler()
	rec := do(h, loginRequest("letmein", "//evil.example"))
	if got := rec.Header().Get("Location"); got != "/" {
		t.Errorf("redirect = %s, want /", got)
	}
}

func TestServiceToken(t *testing.T) {
	_, h := newTestHandler()

	r := httptest.NewRequest(http.MethodGet, "/rates", nil)
	r.Header.Set("X-Preview-Token", "test-secret")
	if rec := do(h, r); rec.Code != http.StatusOK {
		t.Errorf("valid token status = %d, want 200", rec.Code)
	}

	r.Header.Set("X-Preview-Token", "wrong")
	if rec := do(h, r); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token status = %d, want 401", rec.Code)
	}
}

func TestTokenExpiry(t *testing.T) {
	g, _ := newTestHandler()
	start := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return start }
	token := g.makeToken()

	if !g.validToken(token) {
		t.Error("fresh token should be valid")
	}
	if g.validToken(token + "x") {
		t.Error("tampered token should be rejected")
	}
	if g.validToken("garbage") {
		t.Error("garbage should be rejected")
	}

	g.now = func() time.Time { return start.Add(8 * 24 * time.Hour) }
	if g.validToken(token) {
		t.Error("expired token should be rejected")
	}
}
