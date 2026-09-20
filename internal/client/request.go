package client

// Request construction: the headers and the cookie policy every Netflix request
// goes out with.

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/seifreed/netflixcli/internal/cookie"
)

func (c *Client) newReq(method, rawURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("accept-language", c.acceptLanguage())
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
