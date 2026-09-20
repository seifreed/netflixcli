package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/seifreed/netflixcli/internal/config"
)

// Netflix's web app does not send GraphQL documents: it sends the id of a query
// the server already has ("persisted queries"). The ids live in the Akira client
// bundle, which registers each one as
//
//	SomeNameDocument={__meta__:{q:"<uuid>"},kind:"Document",…}
//
// and pins the protocol version in the link that sends them. Both are scraped
// once per build and cached, because the bundle is tens of megabytes.
var (
	bundleURLRe   = regexp.MustCompile(`https://assets\.nflxext\.com/web/ffe/wp/ui/akira/akiraClient\.[0-9a-f]+\.js`)
	persistedOpRe = regexp.MustCompile(`(\w+)Document\s*=\s*\{__meta__:\{q:"([0-9a-fA-F-]{36})"\}`)
	pqVersionRe   = regexp.MustCompile(`persistedQuery\s*=\s*\{id:\w+,version:(\d+)\}`)
)

const (
	manifestFile = "queries.json"
	// defaultPersistedVersion is used when the bundle no longer states one.
	defaultPersistedVersion = 102
	maxBundleBytes          = 64 << 20
)

// queryManifest maps GraphQL operation names to the persisted query ids the
// Netflix gateway accepts, for one Shakti build.
type queryManifest struct {
	Build   string            `json:"build"`
	Version int               `json:"version"`
	Ops     map[string]string `json:"ops"`
}

func (m *queryManifest) id(op string) (string, bool) {
	id, ok := m.Ops[op]
	return id, ok
}

// manifest returns the persisted-query manifest for the current build, reading
// the on-disk cache first and scraping the client bundle only when the cache is
// missing or stale.
func (c *Client) manifest() (*queryManifest, error) {
	ctx, err := c.context()
	if err != nil {
		return nil, err
	}
	if c.queries != nil && c.queries.Build == ctx.BuildID {
		return c.queries, nil
	}
	var cached queryManifest
	if err := config.Load(manifestFile, &cached); err == nil && cached.Build == ctx.BuildID && len(cached.Ops) > 0 {
		c.queries = &cached
		return c.queries, nil
	}
	if ctx.BundleURL == "" {
		return nil, fmt.Errorf("netflix page does not reference a client bundle — cannot resolve GraphQL queries")
	}
	c.logf("indexing Netflix GraphQL queries for build %s (one-off download, cached in %s)", ctx.BuildID, config.Dir())
	fresh, err := c.scrapeManifest(ctx.BundleURL)
	if err != nil {
		return nil, err
	}
	fresh.Build = ctx.BuildID
	if err := config.Save(manifestFile, fresh); err != nil {
		c.logf("could not cache the query manifest (%v) — it will be rebuilt next run", err)
	}
	c.queries = fresh
	return fresh, nil
}

// scrapeManifest downloads the Akira client bundle and extracts every persisted
// query id it registers.
func (c *Client) scrapeManifest(bundleURL string) (*queryManifest, error) {
	u, err := url.Parse(bundleURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Hostname(), "assets.nflxext.com") {
		return nil, fmt.Errorf("refusing to download a client bundle from %q", bundleURL)
	}
	req, err := http.NewRequest("GET", bundleURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", c.UserAgent)
	req.Header.Set("accept", "*/*")
	req.Header.Set("referer", c.BaseURL+"/")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download netflix client bundle: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Body: "client bundle download failed"}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBundleBytes))
	if err != nil {
		return nil, fmt.Errorf("read netflix client bundle: %w", err)
	}
	return parseManifest(string(data))
}

func parseManifest(bundle string) (*queryManifest, error) {
	m := &queryManifest{Version: defaultPersistedVersion, Ops: map[string]string{}}
	for _, match := range persistedOpRe.FindAllStringSubmatch(bundle, -1) {
		m.Ops[match[1]] = match[2]
	}
	if len(m.Ops) == 0 {
		return nil, fmt.Errorf("no persisted GraphQL queries found in the netflix client bundle")
	}
	if v := pqVersionRe.FindStringSubmatch(bundle); v != nil {
		if n, err := strconv.Atoi(v[1]); err == nil && n > 0 {
			m.Version = n
		}
	}
	return m, nil
}

// Operations lists the GraphQL operations the current build exposes, sorted by
// name — useful for discovering what the CLI could call.
func (c *Client) Operations() (map[string]string, error) {
	m, err := c.manifest()
	if err != nil {
		return nil, err
	}
	return m.Ops, nil
}
