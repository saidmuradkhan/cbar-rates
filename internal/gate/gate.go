// Package gate keeps the site behind a simple login page until it is ready for review.
package gate

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	CookieName = "preview_session"
	cookieTTL  = 7 * 24 * time.Hour
)

var openPaths = map[string]bool{"/health": true, "/login": true, "/logout": true}

type Gate struct {
	User     string
	Password string
	Secret   string
	now      func() time.Time
}

// FromEnv returns nil when PREVIEW_USER or PREVIEW_PASSWORD is not set, which means the site is public.
func FromEnv() *Gate {
	user, password := os.Getenv("PREVIEW_USER"), os.Getenv("PREVIEW_PASSWORD")
	if user == "" || password == "" {
		return nil
	}
	secret := os.Getenv("PREVIEW_SECRET")
	if secret == "" {
		secret = password
	}
	return New(user, password, secret)
}

func New(user, password, secret string) *Gate {
	return &Gate{User: user, Password: password, Secret: secret, now: time.Now}
}

func (g *Gate) Wrap(next http.Handler) http.Handler {
	if g == nil {
		return next
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", g.showLogin)
	mux.HandleFunc("POST /login", g.submitLogin)
	mux.HandleFunc("GET /logout", g.logout)
	mux.Handle("/", g.protect(next))
	return mux
}

func (g *Gate) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.allows(r) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.Path), http.StatusSeeOther)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"login required"}` + "\n"))
	})
}

func (g *Gate) allows(r *http.Request) bool {
	if openPaths[r.URL.Path] {
		return true
	}
	if token := r.Header.Get("X-Preview-Token"); token != "" && equal(token, g.Secret) {
		return true
	}
	cookie, err := r.Cookie(CookieName)
	return err == nil && g.validToken(cookie.Value)
}

func (g *Gate) makeToken() string {
	expires := g.now().Add(cookieTTL).Unix()
	return strconv.FormatInt(expires, 10) + "." + g.sign(expires)
}

func (g *Gate) validToken(token string) bool {
	expiresText, signature, found := strings.Cut(token, ".")
	if !found {
		return false
	}
	expires, err := strconv.ParseInt(expiresText, 10, 64)
	if err != nil || g.now().Unix() > expires {
		return false
	}
	return equal(signature, g.sign(expires))
}

func (g *Gate) sign(expires int64) string {
	mac := hmac.New(sha256.New, []byte(g.Secret))
	mac.Write([]byte(g.User + ":" + strconv.FormatInt(expires, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (g *Gate) showLogin(w http.ResponseWriter, r *http.Request) {
	renderLogin(w, http.StatusOK, safeNext(r.URL.Query().Get("next")), false)
}

func (g *Gate) submitLogin(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	userOK := equal(r.FormValue("username"), g.User)
	passwordOK := equal(r.FormValue("password"), g.Password)
	if !userOK || !passwordOK {
		renderLogin(w, http.StatusUnauthorized, next, true)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    g.makeToken(),
		Path:     "/",
		MaxAge:   int(cookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (g *Gate) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/"
}

func renderLogin(w http.ResponseWriter, status int, next string, failed bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	loginPage.Execute(w, map[string]any{"Next": next, "Failed": failed})
}

var loginPage = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Sign in · cbar-rates</title>
<style>
  body { font-family: system-ui, sans-serif; background: #f4f5f7; margin: 0;
         min-height: 100vh; display: grid; place-items: center; color: #1f2328; }
  form { background: #fff; padding: 2rem; border-radius: 12px; width: min(320px, 90vw);
         box-shadow: 0 4px 24px rgba(0, 0, 0, .08); display: grid; gap: .75rem; }
  h1 { font-size: 1.2rem; margin: 0 0 .5rem; }
  input { padding: .6rem; border: 1px solid #d0d7de; border-radius: 6px; font-size: 1rem; }
  button { padding: .65rem; border: 0; border-radius: 6px; background: #1f6feb;
           color: #fff; font-size: 1rem; cursor: pointer; }
  .error { color: #cf222e; margin: 0; font-size: .9rem; }
  .hint { color: #656d76; font-size: .8rem; margin: 0; }
</style>
</head>
<body>
<form method="post" action="/login">
  <h1>cbar-rates</h1>
  <p class="hint">This preview is private for now.</p>
  {{if .Failed}}<p class="error">Wrong username or password.</p>{{end}}
  <input type="hidden" name="next" value="{{.Next}}">
  <input name="username" placeholder="Username" autocomplete="username" required autofocus>
  <input name="password" type="password" placeholder="Password" autocomplete="current-password" required>
  <button type="submit">Sign in</button>
</form>
</body>
</html>
`))
