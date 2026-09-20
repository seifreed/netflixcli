// Package cookie contains the shared validation rules for Netflix session cookies.
package cookie

import (
	"net/http"
	"strings"
)

const netflixDomain = "netflix.com"

// MaxHeaderBytes bounds the size of a session Cookie header accepted by the CLI.
const MaxHeaderBytes = 64 << 10

// IsNetflixHost reports whether host is the Netflix apex or a subdomain.
func IsNetflixHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == netflixDomain || strings.HasSuffix(host, "."+netflixDomain)
}

// ValidHeader reports whether raw is a non-empty HTTP Cookie header.
func ValidHeader(raw string) bool {
	if len(raw) > MaxHeaderBytes || strings.TrimSpace(raw) == "" {
		return false
	}
	cookies, err := http.ParseCookie(raw)
	return err == nil && len(cookies) > 0
}

// ValidPair reports whether a browser cookie name/value pair is safe to serialize.
func ValidPair(name, value string) bool {
	parsed, err := http.ParseCookie(name + "=" + value)
	return err == nil && len(parsed) == 1 && parsed[0].Name == name && parsed[0].Value == value
}

// LooksAuthenticated reports whether raw carries a signed-in Netflix session.
// NetflixId is the membership cookie; without it the site serves the logged-out
// marketing pages instead of the member API.
func LooksAuthenticated(raw string) bool {
	return Value(raw, "NetflixId") != ""
}

// Value returns the value of the named cookie in a Cookie header, or "".
func Value(raw, want string) string {
	for _, part := range strings.Split(raw, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(name, want) {
			return value
		}
	}
	return ""
}
