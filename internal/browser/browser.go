// Package browser fetches pages through an already-running Chrome instance.
// It never starts Chrome: the caller supplies a local DevTools Protocol endpoint.
package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// DefaultEndpoint is the conventional local Chrome DevTools endpoint.
const DefaultEndpoint = "http://127.0.0.1:9222"

// Fetcher renders pages through an existing Chrome DevTools session.
type Fetcher struct {
	endpoint string
}

// New validates a local Chrome DevTools endpoint without starting Chrome.
func New(endpoint string) (*Fetcher, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("chrome CDP endpoint must be a local http(s) URL")
	}
	host := u.Hostname()
	if !net.ParseIP(host).IsLoopback() && host != "localhost" {
		return nil, errors.New("chrome CDP endpoint must use localhost or a loopback IP")
	}
	return &Fetcher{endpoint: endpoint}, nil
}

// Fetch navigates a new tab in the existing Chrome and returns its rendered HTML.
// An interstitial is allowed to settle in that tab until timeout, so the user can
// clear it in the same browser profile the session came from.
func (f *Fetcher) Fetch(rawURL string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, f.endpoint)
	defer allocCancel()
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	defer tabCancel()
	if err := chromedp.Run(tabCtx, chromedp.Navigate(rawURL), chromedp.WaitReady("body"), chromedp.Sleep(2*time.Second)); err != nil {
		return "", fmt.Errorf("navigate through existing Chrome at %s: %w (start Chrome with --remote-debugging-port=9222)", f.endpoint, err)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		var html string
		if err := chromedp.Run(tabCtx, chromedp.OuterHTML("html", &html)); err != nil {
			return "", fmt.Errorf("read page from existing Chrome: %w", err)
		}
		if strings.TrimSpace(html) != "" {
			return html, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("page stayed empty in existing Chrome before timeout")
		case <-ticker.C:
		}
	}
}
