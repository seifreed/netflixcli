// Package client talks to netflix.com the way the web app does, over two paths
// because Netflix serves its data two ways:
//
//   - Browse surfaces come out of the page itself. Netflix server-renders every
//     row it shows as an Apollo cache embedded in the HTML, so one page fetch
//     yields My List, Continue Watching and the editorial rows (apollo.go).
//   - Search, title detail, episodes and every write go to the GraphQL gateway
//     as *persisted* operations: an id, not a query document. The ids change
//     with each Netflix build, so they are scraped from the client bundle and
//     cached (graphql.go, manifest.go).
//
// Requests present Chrome's TLS fingerprint (transport.go) and carry a browser
// cookie lifted with `login --from-browser`, `import-har` or `set-cookie`;
// Netflix serves member data to nothing else. The package never handles a
// password.
//
// Client owns the session. What you can ask for hangs off it as three services
// — Catalog, Library and Account — described in services.go.
package client

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/seifreed/netflixcli/internal/cookie"
)

const (
	// BaseURL is the public site host.
	BaseURL = "https://www.netflix.com"
	// DefaultUA mirrors a current desktop Chrome so requests look like the web app.
	DefaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36"
)

// Client is the API entrypoint. The zero value is not usable — use New.
type Client struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string
	Lang      string // UI language, e.g. "es-ES" (default) or "en"

	// Cookie carries the browser session (NetflixId / SecureNetflixId).
	Cookie string

	// GraphQLURL is the gateway persisted operations are sent to. It defaults to
	// GraphQLEndpoint; tests point it at a stub.
	GraphQLURL string

	// Logf, when set, receives human-readable diagnostics on stderr (nil disables).
	Logf func(format string, args ...any)

	// fetch, when set, renders pages somewhere other than this client's
	// transport — an already-running Chrome, or a stub in tests. See SetFetcher.
	fetch func(url string) (string, error)

	// What the session can be asked for. See services.go.
	Catalog *Catalog
	Library *Library
	Account *Account

	ctx          *shaktiContext // page bootstrap: build id, bundle URL, who is signed in
	queries      *queryManifest // persisted GraphQL query ids for that build
	transportErr error
	warnOnce     sync.Once
}

// SetFetcher routes page loads through a caller-provided renderer, such as an
// already-running Chrome CDP session. URL validation still happens in getText.
func (c *Client) SetFetcher(fetch func(string) (string, error)) {
	c.fetch = fetch
}

func (c *Client) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// New returns a Client with web-app-like defaults. Its HTTP client presents
// Chrome's TLS (JA3) fingerprint via uTLS; if that fails to initialise it falls
// back to the stdlib transport.
func New() *Client {
	hc := &http.Client{Timeout: 30 * time.Second}
	c := &Client{HTTP: hc, BaseURL: BaseURL, GraphQLURL: GraphQLEndpoint, UserAgent: DefaultUA, Lang: "es-ES"}
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && cookie.ValidHeader(c.Cookie) && trustedCookieRequest(via[0].URL.String()) {
			initial := via[0].URL
			if !strings.EqualFold(initial.Scheme, req.URL.Scheme) || !strings.EqualFold(initial.Host, req.URL.Host) {
				return http.ErrUseLastResponse
			}
		}
		return nil
	}
	if tr, err := newChromeTransport(); err == nil {
		hc.Transport = tr
	} else {
		c.transportErr = err
	}
	c.Catalog = &Catalog{c}
	c.Library = &Library{c}
	c.Account = &Account{c}
	return c
}

const maxBodyBytes = 32 << 20

// do sends req, reads the (capped) body, and maps a non-2xx status to *APIError.
func (c *Client) do(req *http.Request) ([]byte, error) {
	c.warnOnce.Do(func() {
		if c.transportErr != nil {
			c.logf("uTLS Chrome fingerprint unavailable (%v) — using the stdlib transport; Netflix is more likely to refuse the request", c.transportErr)
		}
	})
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{
			Status:     resp.StatusCode,
			Body:       string(data),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
		if rejectsSession(resp.StatusCode) {
			return nil, fmt.Errorf("%w (HTTP %d)", ErrSessionRejected, resp.StatusCode)
		}
		return nil, apiErr
	}
	if readErr != nil {
		return nil, fmt.Errorf("read response body from %s: %w", req.URL, readErr)
	}
	return data, nil
}

// getText fetches a URL and returns the raw body, retrying transient throttling
// (429/503). The fetch hook (tests, or a Chrome CDP session) short-circuits the
// HTTP path.
func (c *Client) getText(rawURL string) (string, error) {
	if !c.sameOrigin(rawURL) {
		return "", fmt.Errorf("request URL must belong to the configured Netflix host")
	}
	if c.fetch != nil {
		html, err := c.fetch(rawURL)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(html) == "" {
			return "", &APIError{Status: http.StatusForbidden, Body: "empty page from fetch override"}
		}
		return html, nil
	}
	backoff := defaultRetryBase
	for attempt := 0; ; attempt++ {
		s, err := c.getOnce(rawURL)
		if !isRateLimited(err) || attempt >= maxRetries {
			return s, err
		}
		wait := retryWait(err, backoff)
		status, _ := httpStatus(err)
		c.logf("throttled: HTTP %d on %s — retrying %d/%d in %s", status, rawURL, attempt+1, maxRetries, wait.Round(time.Millisecond))
		time.Sleep(wait)
		backoff *= 2
	}
}

func (c *Client) sameOrigin(raw string) bool {
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return false
	}
	target, err := url.Parse(raw)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return false
	}
	return strings.EqualFold(base.Scheme, target.Scheme) && strings.EqualFold(base.Host, target.Host)
}

func (c *Client) getOnce(rawURL string) (string, error) {
	req, err := c.newReq("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	data, err := c.do(req)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
