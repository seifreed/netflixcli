package client

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// A slice of the Akira bundle in the shape the scraper has to survive: minified,
// with the persisted ids attached as __meta__ and the protocol version pinned in
// the link that sends them.
const bundleSample = `var x=1;i.SearchPageQueryResultsDocument={__meta__:{q:"c068ad18-089b-418f-a3d7-02d34858bfc0"},kind:"Document",` +
	`definitions:[]},i.MiniModalQueryDocument={__meta__:{q:"e68167bb-f301-45a0-94e6-33f47aa333ef"},kind:"Document"};` +
	`o&&(n.extensions.persistedQuery={id:o,version:102},n.setContext({}));`

func TestParseManifest(t *testing.T) {
	m, err := parseManifest(bundleSample)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if got := len(m.Ops); got != 2 {
		t.Fatalf("found %d operations, want 2", got)
	}
	if id, ok := m.id("SearchPageQueryResults"); !ok || id != "c068ad18-089b-418f-a3d7-02d34858bfc0" {
		t.Errorf("SearchPageQueryResults = %q (found=%v), want the registered uuid", id, ok)
	}
	if m.Version != 102 {
		t.Errorf("version = %d, want the 102 pinned in the bundle", m.Version)
	}
}

func TestParseManifestRejectsBundleWithoutQueries(t *testing.T) {
	if _, err := parseManifest("var x=1;"); err == nil {
		t.Fatal("want an error when the bundle registers no persisted queries")
	}
}

func TestParseManifestFallsBackToDefaultVersion(t *testing.T) {
	m, err := parseManifest(`i.FooDocument={__meta__:{q:"c068ad18-089b-418f-a3d7-02d34858bfc0"},kind:"Document"}`)
	if err != nil {
		t.Fatalf("parseManifest: %v", err)
	}
	if m.Version != defaultPersistedVersion {
		t.Errorf("version = %d, want the %d fallback", m.Version, defaultPersistedVersion)
	}
}

func TestScrapeManifestRefusesForeignHost(t *testing.T) {
	c := New()
	if _, err := c.scrapeManifest("https://evil.example/akiraClient.deadbeef.js"); err == nil {
		t.Fatal("want a refusal when the bundle is not served by Netflix's asset host")
	}
}

// roundTripperFunc answers a request without a network.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The client bundle is a public asset on a CDN. It is the one host this CLI
// talks to that is not the member site or its gateway, and the session cookie
// has no business going there. The request is built by hand rather than through
// newReq, so nothing else enforces it.
func TestBundleDownloadCarriesNoCookie(t *testing.T) {
	var got http.Header
	c := New()
	c.Cookie = "NetflixId=secret; SecureNetflixId=alsosecret"
	c.useTransport(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		got = r.Header.Clone()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`SomeDocument={__meta__:{q:"11111111-2222-3333-4444-555555555555"}}`)),
			Header:     http.Header{},
		}, nil
	}))

	manifest, err := c.scrapeManifest("https://assets.nflxext.com/web/ffe/wp/ui/akira/akiraClient.abc123.js")
	if err != nil {
		t.Fatalf("scrapeManifest: %v", err)
	}
	if len(manifest.Ops) != 1 {
		t.Errorf("scraped %d operations, want 1", len(manifest.Ops))
	}
	if cookie := got.Get("cookie"); cookie != "" {
		t.Errorf("the bundle request carried the session cookie: %q", cookie)
	}
	if got.Get("user-agent") == "" {
		t.Error("the bundle request lost the browser user agent")
	}
}

// And it is fetched from the CDN or not at all: the bundle URL comes out of a
// page, and a page that names somewhere else must not be followed.
func TestBundleDownloadRefusesAnotherHost(t *testing.T) {
	c := New()
	c.useTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Error("a request left for a bundle URL that should have been refused")
		return nil, errors.New("unreachable")
	}))
	for _, raw := range []string{
		"https://evil.example/akiraClient.abc.js",
		"http://assets.nflxext.com/akiraClient.abc.js",
		"https://assets.nflxext.com.evil.example/x.js",
		"file:///etc/passwd",
	} {
		if _, err := c.scrapeManifest(raw); err == nil {
			t.Errorf("scrapeManifest(%q) was allowed", raw)
		}
	}
}
