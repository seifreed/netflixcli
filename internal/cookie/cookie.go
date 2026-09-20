// Package cookie contains the shared validation rules for Netflix session cookies.
package cookie

import (
	"net/http"
	"sort"
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

// Parse reads a Cookie header into its name/value pairs. A part that carries no
// "=" is not a cookie and is skipped.
func Parse(header string) map[string]string {
	jar := map[string]string{}
	for _, part := range strings.Split(header, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && name != "" {
			jar[name] = value
		}
	}
	return jar
}

// Header renders cookie pairs as "n1=v1; n2=v2", sorted by name so the same jar
// always produces the same header, and dropping any pair that could not be
// carried by one.
func Header(jar map[string]string) string {
	names := make([]string, 0, len(jar))
	for name := range jar {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		if ValidPair(name, jar[name]) {
			parts = append(parts, name+"="+jar[name])
		}
	}
	return strings.Join(parts, "; ")
}

// Value returns the value of the named cookie in a Cookie header, or "".
func Value(raw, want string) string {
	for name, value := range Parse(raw) {
		if strings.EqualFold(name, want) {
			return value
		}
	}
	return ""
}

// Merge applies Set-Cookie updates to a Cookie header. A cookie the server
// expires (MaxAge < 0) is dropped.
func Merge(header string, updates []*http.Cookie) string {
	jar := Parse(header)
	for _, c := range updates {
		if c == nil || c.Name == "" {
			continue
		}
		if c.MaxAge < 0 || !ValidPair(c.Name, c.Value) {
			delete(jar, c.Name)
			continue
		}
		jar[c.Name] = c.Value
	}
	return Header(jar)
}
