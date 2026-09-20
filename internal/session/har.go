package session

import (
	"encoding/json"
	"fmt"
	neturl "net/url"
	"strings"

	"github.com/seifreed/netflixcli/internal/cookie"
)

// MaxHARBytes bounds memory used while importing a browser export.
const MaxHARBytes = 32 << 20

// ParseHAR scans a HAR (DevTools → Network → "Save all as HAR with sensitive
// data", exported while signed in to netflix.com) for the freshest request
// carrying the Netflix session cookie. Only request Cookie headers are read;
// request bodies are never touched.
func ParseHAR(data []byte) (Session, error) {
	if len(data) > MaxHARBytes {
		return Session{}, fmt.Errorf("HAR is too large (maximum %d MiB)", MaxHARBytes>>20)
	}
	var har struct {
		Log struct {
			Entries []struct {
				Request struct {
					URL     string `json:"url"`
					Headers []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"headers"`
				} `json:"request"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		return Session{}, fmt.Errorf("parse HAR JSON: %w", err)
	}

	var s Session
	var sawRequest, sawCookieHeader bool
	var freshestCookie string
	for _, e := range har.Log.Entries {
		u, perr := neturl.Parse(e.Request.URL)
		if perr != nil || !strings.EqualFold(u.Scheme, "https") || !cookie.IsNetflixHost(u.Hostname()) {
			continue
		}
		sawRequest = true
		for _, h := range e.Request.Headers {
			if !strings.EqualFold(h.Name, "cookie") {
				continue
			}
			sawCookieHeader = true
			freshestCookie = h.Value // freshest cookie header, authed or not
			if cookie.LooksAuthenticated(h.Value) {
				s.Cookie = h.Value // last (freshest) session-bearing request wins
			}
		}
	}
	if s.Cookie == "" {
		s.Cookie = freshestCookie // accept a cookie without NetflixId rather than nothing
	}
	if s.Cookie == "" {
		return s, harNoCookieErr(sawRequest, sawCookieHeader)
	}
	return s, nil
}

func harNoCookieErr(sawReq, sawCookie bool) error {
	switch {
	case !sawReq:
		return fmt.Errorf("this HAR has no netflix.com requests — export it while signed in to netflix.com")
	case !sawCookie:
		return fmt.Errorf("HAR sanitised (no Cookie headers) — re-export with: " +
			"DevTools → Network → right-click a request → " +
			"\"Save all as HAR with sensitive data\" (not the ⤓ button, which sanitises)")
	default:
		return fmt.Errorf("no usable cookie found in HAR — export it while signed in to netflix.com")
	}
}
