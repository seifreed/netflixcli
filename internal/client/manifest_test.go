package client

import "testing"

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
