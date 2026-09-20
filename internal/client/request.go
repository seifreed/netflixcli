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

// acceptLanguage asks for the configured language, then its base, then English,
// each once and in descending preference. Listing a tag twice — which is what
// `--lang en` and `--lang en-GB` used to produce — is not a language a server
// can weigh.
func (c *Client) acceptLanguage() string {
	lang := strings.TrimSpace(c.Lang)
	if lang == "" {
		lang = defaultLang
	}
	tags := []string{lang}
	if base, _, hasRegion := strings.Cut(lang, "-"); hasRegion && !listsTag(tags, base) {
		tags = append(tags, base)
	}
	if !listsTag(tags, "en") {
		tags = append(tags, "en")
	}
	header := tags[0]
	for i, tag := range tags[1:] {
		header += fmt.Sprintf(",%s;q=0.%d", tag, 9-i)
	}
	return header
}

func listsTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}

func trustedCookieRequest(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "https") && cookie.IsNetflixHost(u.Hostname())
}
