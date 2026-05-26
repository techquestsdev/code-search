package middleware

import (
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
)

// RealIP returns chi-compatible middleware that rewrites r.RemoteAddr to the
// real client IP by walking the X-Forwarded-For chain — but ONLY when the
// immediate TCP peer is in trustedProxies. This closes the spoofing vector
// that caused chi's own middleware.RealIP to be deprecated
// (GHSA-3fxj-6jh8-hvhx, GHSA-rjr7-jggh-pgcp, GHSA-9g5q-2w5x-hmxf).
//
// If trustedProxies is empty, the middleware is a no-op — r.RemoteAddr keeps
// the actual TCP peer address. That is the safe default: better to log the
// proxy IP than to trust a client-supplied header from an unknown network.
func RealIP(trustedProxies []*net.IPNet) func(http.Handler) http.Handler {
	if len(trustedProxies) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client := realClientIP(r, trustedProxies); client != "" {
				r.RemoteAddr = client
			}

			next.ServeHTTP(w, r)
		})
	}
}

func realClientIP(r *http.Request, trusted []*net.IPNet) string {
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}

	peerIP := net.ParseIP(peerHost)
	if peerIP == nil || !ipInAny(peerIP, trusted) {
		return ""
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return ""
	}

	parts := strings.Split(xff, ",")
	for _, raw := range slices.Backward(parts) {
		candidate := strings.TrimSpace(raw)

		ip := net.ParseIP(candidate)
		if ip == nil {
			continue
		}

		if !ipInAny(ip, trusted) {
			return candidate
		}
	}

	return ""
}

func ipInAny(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}

	return false
}

// ParseTrustedProxies parses CIDR notation strings into []*net.IPNet.
// Bare IPv4/IPv6 addresses are accepted and converted to single-host CIDRs.
func ParseTrustedProxies(in []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(in))

	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		if _, n, err := net.ParseCIDR(s); err == nil {
			out = append(out, n)
			continue
		}

		if ip := net.ParseIP(s); ip != nil {
			suffix := "/32"
			if ip.To4() == nil {
				suffix = "/128"
			}

			if _, n, err := net.ParseCIDR(s + suffix); err == nil {
				out = append(out, n)
				continue
			}
		}

		return nil, fmt.Errorf("invalid trusted_proxy value %q: expected CIDR or IP", s)
	}

	return out, nil
}
