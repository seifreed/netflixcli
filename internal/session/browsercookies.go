// Package session handles browser-cookie import and the local Netflix session cache.
package session

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all" // register Chrome/Firefox/Safari/Edge/Brave finders
	"github.com/seifreed/netflixcli/internal/cookie"
)

// cookieDomain is the substring every netflix cookie's domain carries; used to
// pull just the site's cookies out of a browser's (large) cookie store.
const cookieDomain = "netflix.com"

type nameVal struct{ name, val string }

type browserCookie struct {
	browser string
	name    string
	value   string
}

// traverseCookies is indirected so cookie selection can be tested without
// depending on whichever browsers are installed on the host.
var traverseCookies = kooky.TraverseCookies

// CookiesFromBrowser reads the Netflix session straight out of a browser's
// cookie store: the user signs in to netflix.com normally and the CLI lifts the
// resulting cookies (HttpOnly NetflixId included, since this reads the decrypted
// store and not page JS). browser
// filters to one of chrome|chromium|firefox|safari|edge|brave; "" reads every
// installed browser.
func CookiesFromBrowser(browser string) (Session, error) {
	browser = strings.ToLower(strings.TrimSpace(browser))
	if browser != "" && !supportedBrowser(browser) {
		return Session{}, fmt.Errorf("unsupported browser %q (want chrome|chromium|firefox|safari|edge|brave)", browser)
	}

	// Collect netflix cookies grouped BY BROWSER, skipping per-store failures so
	// an uninstalled browser doesn't abort the read of the one actually used.
	var cookies []browserCookie
	for c, err := range traverseCookies(context.Background(), kooky.DomainContains(cookieDomain)) {
		if err != nil || c == nil || c.Name == "" || c.Value == "" {
			continue
		}
		if !cookie.IsNetflixHost(strings.TrimPrefix(strings.TrimSpace(c.Domain), ".")) || !cookie.ValidPair(c.Name, c.Value) {
			continue
		}
		bname := ""
		if c.Browser != nil {
			bname = strings.ToLower(c.Browser.Browser())
		}
		if browser != "" && bname != browser {
			continue
		}
		cookies = append(cookies, browserCookie{browser: bname, name: c.Name, value: c.Value})
	}

	if cookie, ok := pickSessionCookie(cookies, browser); ok {
		return Session{Cookie: cookie}, nil
	}

	where := "your browser"
	if browser != "" {
		where = browser
	}
	return Session{}, fmt.Errorf("no netflix cookies found in %s — "+
		"sign in to www.netflix.com in that browser first "+
		"(or pass --from-browser <chrome|firefox|safari|edge|brave>)", where)
}

func supportedBrowser(browser string) bool {
	switch browser {
	case "chrome", "chromium", "firefox", "safari", "edge", "brave":
		return true
	default:
		return false
	}
}

// pickSessionCookie keeps cookies from separate browser profiles isolated and
// prefers the first profile carrying a signed-in NetflixId.
func pickSessionCookie(cookies []browserCookie, want string) (string, bool) {
	stores := map[string]map[string]string{}
	var storeOrder []string
	for _, c := range cookies {
		if want != "" && c.browser != want {
			continue
		}
		if stores[c.browser] == nil {
			stores[c.browser] = map[string]string{}
			storeOrder = append(storeOrder, c.browser)
		}
		stores[c.browser][c.name] = c.value
	}
	var fallback string
	for _, bname := range storeOrder {
		store := stores[bname]
		pairs := make([]nameVal, 0, len(store))
		for name, value := range store {
			pairs = append(pairs, nameVal{name, value})
		}
		header := buildCookieHeader(pairs)
		if cookie.LooksAuthenticated(header) {
			return header, true
		}
		if fallback == "" {
			fallback = header
		}
	}
	return fallback, fallback != ""
}

// buildCookieHeader renders cookie name/value pairs as "n1=v1; n2=v2", sorted for
// a deterministic result.
func buildCookieHeader(pairs []nameVal) string {
	sorted := append([]nameVal(nil), pairs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	parts := make([]string, 0, len(sorted))
	for _, p := range sorted {
		if cookie.ValidPair(p.name, p.val) {
			parts = append(parts, p.name+"="+p.val)
		}
	}
	return strings.Join(parts, "; ")
}
