package session

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/browserutils/kooky"
)

// stubBrowser is the bit of kooky's BrowserInfo that cookie selection reads.
type stubBrowser struct{ name string }

func (b stubBrowser) Browser() string        { return b.name }
func (b stubBrowser) Profile() string        { return "Default" }
func (b stubBrowser) IsDefaultProfile() bool { return true }
func (b stubBrowser) FilePath() string       { return "" }

func storeCookie(browser, domain, name, value string) *kooky.Cookie {
	c := &kooky.Cookie{Browser: stubBrowser{browser}}
	c.Domain = domain
	c.Name = name
	c.Value = value
	return c
}

// withCookieStore replaces the browser scan for the duration of a test, so
// selection is exercised without depending on which browsers the host has.
func withCookieStore(t *testing.T, cookies ...*kooky.Cookie) {
	t.Helper()
	original := traverseCookies
	traverseCookies = func(context.Context, ...kooky.Filter) kooky.CookieSeq {
		return func(yield func(*kooky.Cookie, error) bool) {
			for _, c := range cookies {
				if !yield(c, nil) {
					return
				}
			}
		}
	}
	t.Cleanup(func() { traverseCookies = original })
}

func cookieNames(header string) []string {
	var names []string
	for _, part := range strings.Split(header, ";") {
		name, _, _ := strings.Cut(strings.TrimSpace(part), "=")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// A browser profile that is not signed in yields a header without NetflixId;
// the one that is signed in must win, whichever order they are scanned in.
func TestCookiesFromBrowserPrefersTheSignedInStore(t *testing.T) {
	withCookieStore(t,
		storeCookie("firefox", ".netflix.com", "nfvdid", "anonymous"),
		storeCookie("chrome", ".netflix.com", "nfvdid", "signed-in"),
		storeCookie("chrome", ".netflix.com", "NetflixId", "member"),
	)
	s, err := CookiesFromBrowser("")
	if err != nil {
		t.Fatalf("CookiesFromBrowser: %v", err)
	}
	if !strings.Contains(s.Cookie, "NetflixId=member") {
		t.Errorf("cookie = %q, want the signed-in store", s.Cookie)
	}
	if strings.Contains(s.Cookie, "anonymous") {
		t.Errorf("cookie = %q, mixes two browsers' stores", s.Cookie)
	}
}

func TestCookiesFromBrowserFiltersToOneBrowser(t *testing.T) {
	withCookieStore(t,
		storeCookie("chrome", ".netflix.com", "NetflixId", "chrome-session"),
		storeCookie("firefox", ".netflix.com", "NetflixId", "firefox-session"),
	)
	s, err := CookiesFromBrowser("firefox")
	if err != nil {
		t.Fatalf("CookiesFromBrowser: %v", err)
	}
	if !strings.Contains(s.Cookie, "firefox-session") {
		t.Errorf("cookie = %q, want the firefox store", s.Cookie)
	}
}

// The scan asks for a domain substring, so a lookalike domain can come back;
// only real Netflix hosts may reach the session.
func TestCookiesFromBrowserRejectsLookalikeDomains(t *testing.T) {
	withCookieStore(t,
		storeCookie("chrome", "netflix.com.evil.example", "NetflixId", "stolen"),
		storeCookie("chrome", "evilnetflix.com", "NetflixId", "stolen-too"),
	)
	if _, err := CookiesFromBrowser(""); err == nil {
		t.Fatal("want an error when no genuine Netflix cookie was found")
	}
}

func TestCookiesFromBrowserSkipsUnusablePairs(t *testing.T) {
	withCookieStore(t,
		storeCookie("chrome", ".netflix.com", "", "no-name"),
		storeCookie("chrome", ".netflix.com", "broken", `quo"te`),
		storeCookie("chrome", ".netflix.com", "NetflixId", "member"),
	)
	s, err := CookiesFromBrowser("")
	if err != nil {
		t.Fatalf("CookiesFromBrowser: %v", err)
	}
	if got := cookieNames(s.Cookie); len(got) != 1 || got[0] != "NetflixId" {
		t.Errorf("cookie names = %v, want only NetflixId", got)
	}
	if _, err := http.ParseCookie(s.Cookie); err != nil {
		t.Errorf("built an unparseable Cookie header %q: %v", s.Cookie, err)
	}
}

func TestCookiesFromBrowserRejectsUnknownBrowser(t *testing.T) {
	if _, err := CookiesFromBrowser("netscape"); err == nil {
		t.Fatal("want an error for a browser the CLI cannot read")
	}
}

func TestCookiesFromBrowserReportsAnEmptyStore(t *testing.T) {
	withCookieStore(t)
	_, err := CookiesFromBrowser("chrome")
	if err == nil {
		t.Fatal("want an error when the store has no Netflix cookies")
	}
	if !strings.Contains(err.Error(), "sign in") {
		t.Errorf("error %q does not say what to do about it", err)
	}
}

// The header is sorted so the same store always produces the same session file.
func TestBuildCookieHeaderIsDeterministic(t *testing.T) {
	pairs := []nameVal{{"zeta", "1"}, {"alpha", "2"}, {"NetflixId", "3"}}
	want := "NetflixId=3; alpha=2; zeta=1"
	if got := buildCookieHeader(pairs); got != want {
		t.Errorf("buildCookieHeader = %q, want %q", got, want)
	}
}
