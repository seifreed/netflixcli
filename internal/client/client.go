// Package client is a thin, dependency-light Go client for netflix.com. It
// talks to the same Shakti endpoints the web app uses (the Falcor
// pathEvaluator), presenting Chrome's TLS fingerprint (uTLS) so the requests
// look like the browser's. Netflix serves member data only to a signed-in
// session, so every call carries a browser cookie lifted with
// `login --from-browser`, `import-har` or `set-cookie`.
package client

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
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

	// fetch is an internal test hook; production commands leave it nil and use
	// the configured HTTP transport.
	fetch func(url string) (string, error)

	ctx          *shaktiContext // lazily bootstrapped build id + authURL
	queries      *queryManifest // lazily resolved persisted GraphQL query ids
	transportErr error
	warnOnce     sync.Once
}

// SetFetcher routes page loads through a caller-provided renderer, such as an
// already-running Chrome CDP session. URL validation still happens in GetText.
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
	return c
}

// APIError carries a non-2xx response so callers can branch on it.
type APIError struct {
	Status     int
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	body := truncate(e.Body, 200)
	if isHTMLBody(e.Body) {
		body = fmt.Sprintf("(HTML error page, %d bytes)", len(e.Body))
	}
	return fmt.Sprintf("netflix: HTTP %d: %s", e.Status, body)
}

func isHTMLBody(body string) bool {
	head := strings.ToLower(strings.TrimSpace(body))
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html")
}

// HTTPStatus reports the HTTP status carried by err when it is (or wraps) an
// *APIError. ok is false for any other error.
func HTTPStatus(err error) (status int, ok bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status, true
	}
	return 0, false
}

// NeedsLogin reports whether err means the session is missing or expired, which
// the user fixes by re-importing cookies from the browser.
func NeedsLogin(err error) bool {
	status, ok := HTTPStatus(err)
	return ok && (status == http.StatusUnauthorized || status == http.StatusForbidden)
}

// ErrNoSession is returned before any request when no cookie is configured.
var ErrNoSession = errors.New("no Netflix session — run `netflix login --from-browser chrome` first")

func (c *Client) newReq(method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("accept-language", c.acceptLanguage())
	req.Header.Set("user-agent", c.UserAgent)
	req.Header.Set("referer", c.BaseURL+"/")
	req.Header.Set("upgrade-insecure-requests", "1")
	if c.Cookie != "" && trustedCookieRequest(rawURL) && cookie.ValidHeader(c.Cookie) {
		req.Header.Set("cookie", c.Cookie)
	}
	return req, nil
}

func (c *Client) acceptLanguage() string {
	lang := strings.TrimSpace(c.Lang)
	if lang == "" {
		lang = "es-ES"
	}
	base, _, _ := strings.Cut(lang, "-")
	if base == lang {
		return fmt.Sprintf("%s;q=0.9,en;q=0.8", lang)
	}
	return fmt.Sprintf("%s,%s;q=0.9,en;q=0.8", lang, base)
}

func trustedCookieRequest(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "https") && cookie.IsNetflixHost(u.Hostname())
}

// Automatic backoff on throttling (HTTP 429/503).
const (
	maxRetries       = 3
	defaultRetryBase = 500 * time.Millisecond
	maxRetryWait     = 10 * time.Second
)

func isRateLimited(err error) bool {
	status, ok := HTTPStatus(err)
	return ok && (status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable)
}

func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return -1
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(h); err == nil {
		if wait := time.Until(at); wait > 0 {
			return wait
		}
		return 0
	}
	return -1
}

func retryWait(err error, backoff time.Duration) time.Duration {
	var ae *APIError
	if errors.As(err, &ae) && ae.RetryAfter >= 0 {
		return capWait(ae.RetryAfter)
	}
	return capWait(backoff + time.Duration(rand.Int63n(int64(250*time.Millisecond))))
}

func capWait(d time.Duration) time.Duration {
	if d > maxRetryWait {
		return maxRetryWait
	}
	if d < 0 {
		return 0
	}
	return d
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
		return nil, &APIError{
			Status:     resp.StatusCode,
			Body:       string(data),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	if readErr != nil {
		return nil, fmt.Errorf("read response body from %s: %w", req.URL, readErr)
	}
	return data, nil
}

// GetText fetches a URL and returns the raw body, retrying transient throttling
// (429/503). The fetch hook (tests, or a Chrome CDP session) short-circuits the
// HTTP path.
func (c *Client) GetText(rawURL string) (string, error) {
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
		status, _ := HTTPStatus(err)
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
