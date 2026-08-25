package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStashRefRegex(t *testing.T) {
	valid := []string{"stash@{0}", "stash@{12}", "stash@{999}"}
	for _, r := range valid {
		if !stashRefRe.MatchString(r) {
			t.Errorf("expected valid: %q", r)
		}
	}
	invalid := []string{
		"--upload-pack=evil",
		"stash@{0} --exec=x",
		"stash@{-1}",
		"stash",
		"stash@{}",
		"refs/heads/main",
		"stash@{0};rm -rf /",
		"",
	}
	for _, r := range invalid {
		if stashRefRe.MatchString(r) {
			t.Errorf("expected rejected: %q", r)
		}
	}
}

func TestHostAllowlist(t *testing.T) {
	const port = 3001
	handler := hostAllowlist(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), port)

	req := httptest.NewRequest("GET", "http://127.0.0.1:3001/api/stashes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("loopback host should pass, got %d", rec.Code)
	}

	req = httptest.NewRequest("GET", "http://localhost:3001/api/stashes", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("localhost should pass, got %d", rec.Code)
	}

	for _, host := range []string{"evil.com", "127.0.0.1:9999", ""} {
		req = httptest.NewRequest("GET", "http://127.0.0.1:3001/api/stashes", nil)
		req.Host = host
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("host %q should be forbidden, got %d", host, rec.Code)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("missing Content-Security-Policy")
	}
}

func TestCsrfGuard(t *testing.T) {
	const port = 3001
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := hostAllowlist(csrfGuard(next, port), port)

	cases := []struct {
		name     string
		method   string
		origin   string
		xrw      string
		host     string
		wantCode int
	}{
		{"same-origin post with header", "POST", "http://127.0.0.1:3001", "gstash", "127.0.0.1:3001", 200},
		{"localhost origin post with header", "POST", "http://localhost:3001", "gstash", "localhost:3001", 200},
		{"no origin no header (old curl)", "POST", "", "", "127.0.0.1:3001", 403},
		{"no origin but header (curl)", "POST", "", "gstash", "127.0.0.1:3001", 200},
		{"cross-origin form post", "POST", "http://attacker.com", "", "localhost:3001", 403},
		{"cross-origin with forged-ish header still blocked by origin", "POST", "https://evil.example", "gstash", "127.0.0.1:3001", 403},
		{"empty origin header bypass attempt", "POST", "", "gstash", "localhost:3001", 200},
		{"empty origin no header bypass attempt", "POST", "", "", "localhost:3001", 403},
		{"get unaffected by origin and header", "GET", "http://attacker.com", "", "127.0.0.1:3001", 200},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "http://127.0.0.1:3001/api/pop?ref=stash@{0}", nil)
			req.Host = c.host
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			if c.xrw != "" {
				req.Header.Set("X-Requested-With", c.xrw)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Errorf("%s %s origin=%q xrw=%q host=%q: got %d want %d", c.method, "/api/pop", c.origin, c.xrw, c.host, rec.Code, c.wantCode)
			}
		})
	}
}
