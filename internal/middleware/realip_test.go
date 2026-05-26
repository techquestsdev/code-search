package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealIP_NoTrustedProxies_IsNoOp(t *testing.T) {
	captured := ""
	h := RealIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.RemoteAddr
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	r.Header.Set("X-Forwarded-For", "9.9.9.9")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if captured != "1.2.3.4:5555" {
		t.Fatalf("expected RemoteAddr untouched, got %q", captured)
	}
}

func TestRealIP_TrustedPeer_UsesXFF(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}

	captured := ""
	h := RealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.RemoteAddr
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.5.6:5555"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if captured != "203.0.113.7" {
		t.Fatalf("expected client IP from XFF, got %q", captured)
	}
}

func TestRealIP_UntrustedPeer_IgnoresXFF(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}

	captured := ""
	h := RealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.RemoteAddr
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "8.8.8.8:5555"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if captured != "8.8.8.8:5555" {
		t.Fatalf("expected RemoteAddr untouched for untrusted peer, got %q", captured)
	}
}

func TestRealIP_ChainOfTrustedProxies(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8", "172.16.0.0/12"})
	if err != nil {
		t.Fatal(err)
	}

	captured := ""
	h := RealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.RemoteAddr
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.5.6:5555"
	// Chain: client → ext LB (untrusted) → internal LB (trusted) → peer (trusted)
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 172.16.0.1, 10.0.5.1")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if captured != "203.0.113.7" {
		t.Fatalf("expected first untrusted IP from right, got %q", captured)
	}
}

func TestRealIP_AllForwardersTrusted_NoMatch(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}

	captured := ""
	h := RealIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.RemoteAddr
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.5.6:5555"
	r.Header.Set("X-Forwarded-For", "10.0.5.1, 10.0.5.2")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if captured != "10.0.5.6:5555" {
		t.Fatalf("expected RemoteAddr untouched when no untrusted hop, got %q", captured)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	cases := []struct {
		in     []string
		wantOK bool
	}{
		{[]string{"10.0.0.0/8"}, true},
		{[]string{"::1/128"}, true},
		{[]string{"192.168.1.1"}, true}, // bare IP → /32
		{[]string{"2001:db8::1"}, true}, // bare IPv6 → /128
		{[]string{""}, true},            // empty entries skipped
		{[]string{"not-an-ip"}, false},
		{[]string{"10.0.0.0/99"}, false},
	}

	for _, c := range cases {
		_, err := ParseTrustedProxies(c.in)
		if (err == nil) != c.wantOK {
			t.Errorf("ParseTrustedProxies(%v): err=%v, wantOK=%v", c.in, err, c.wantOK)
		}
	}
}
