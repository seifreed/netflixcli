package client

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// shaktiContext is what the CLI needs out of the page bootstrap: the build the
// persisted queries belong to, where to find them, and who the session is.
type shaktiContext struct {
	BuildID   string
	BundleURL string // Akira client bundle, where the persisted query ids live
	User      UserInfo
	Profile   Profile // the profile the session is acting as
}

// UserInfo is the account summary Netflix embeds in every page. GUID identifies
// the account, not the profile — the profile is a separate id (see Profile).
type UserInfo struct {
	Name             string `json:"name"`
	AccountOwnerName string `json:"accountOwnerName"`
	GUID             string `json:"guid"`
	MembershipStatus string `json:"membershipStatus"`
	CountryOfSignup  string `json:"countryOfSignup"`
	CurrentCountry   string `json:"currentCountry"`
	MemberSince      string `json:"memberSince"`
	IsKids           bool   `json:"isKids"`
	NumProfiles      int    `json:"numProfiles"`
}

// reactContextRe captures the JSON blob Netflix assigns to netflix.reactContext
// in the page bootstrap script.
var reactContextRe = regexp.MustCompile(`(?s)netflix\.reactContext\s*=\s*(\{.*?\});\s*</script>`)

// jsHexEscapeRe matches the \xNN escapes Netflix uses inside that blob; they are
// valid JavaScript string escapes but not valid JSON.
var jsHexEscapeRe = regexp.MustCompile(`\\x([0-9a-fA-F]{2})`)

// parseReactContext extracts the bootstrap model from a Netflix HTML page.
func parseReactContext(html string) (*shaktiContext, error) {
	m := reactContextRe.FindStringSubmatch(html)
	if m == nil {
		return nil, fmt.Errorf("netflix bootstrap not found in page (not signed in, or the page layout changed)")
	}
	// \xNN → \u00NN so encoding/json accepts the blob verbatim.
	blob := jsHexEscapeRe.ReplaceAllString(m[1], `\u00$1`)
	var ctx struct {
		Models struct {
			ServerDefs struct {
				Data struct {
					BuildIdentifier string `json:"BUILD_IDENTIFIER"`
				} `json:"data"`
			} `json:"serverDefs"`
			UserInfo struct {
				Data UserInfo `json:"data"`
			} `json:"userInfo"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(blob), &ctx); err != nil {
		return nil, fmt.Errorf("parse netflix bootstrap: %w", err)
	}
	build := ctx.Models.ServerDefs.Data.BuildIdentifier
	if build == "" {
		return nil, fmt.Errorf("netflix bootstrap carries no build identifier")
	}
	return &shaktiContext{
		BuildID:   build,
		BundleURL: bundleURLRe.FindString(html),
		User:      ctx.Models.UserInfo.Data,
	}, nil
}

// context bootstraps the page state once per process — the build, the client
// bundle and who is signed in — by loading a member page with the cookie.
func (c *Client) context() (*shaktiContext, error) {
	if c.ctx != nil {
		return c.ctx, nil
	}
	if c.Cookie == "" {
		return nil, ErrNoSession
	}
	html, err := c.getText(c.BaseURL + "/browse")
	if err != nil {
		return nil, err
	}
	ctx, err := parseReactContext(html)
	if err != nil {
		return nil, err
	}
	// The same page carries the profile cache, so the active profile costs no
	// extra request.
	if cache, cacheErr := parseApolloCache(html); cacheErr == nil {
		ctx.Profile = cache.currentProfile()
	}
	c.ctx = ctx
	return ctx, nil
}

// Whoami returns the account summary carried by the current session.
func (s *Account) Whoami() (UserInfo, error) {
	ctx, err := s.client.context()
	if err != nil {
		return UserInfo{}, err
	}
	if ctx.User.MembershipStatus == "" || ctx.User.MembershipStatus == "ANONYMOUS" {
		return ctx.User, fmt.Errorf("the session is not signed in (membership status %q) — re-import cookies with `netflix login --from-browser chrome`", ctx.User.MembershipStatus)
	}
	return ctx.User, nil
}
