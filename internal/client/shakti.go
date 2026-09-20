package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// shaktiContext is what the CLI needs out of the page bootstrap: the Shakti
// build id (part of the API path) and the authURL token every write and
// pathEvaluator call must echo back.
type shaktiContext struct {
	BuildID   string
	AuthURL   string
	BundleURL string // Akira client bundle, where the persisted query ids live
	User      UserInfo
}

// UserInfo is the account summary Netflix embeds in every page.
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
				Data struct {
					UserInfo
					AuthURL string `json:"authURL"`
				} `json:"data"`
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
		AuthURL:   ctx.Models.UserInfo.Data.AuthURL,
		BundleURL: bundleURLRe.FindString(html),
		User:      ctx.Models.UserInfo.Data.UserInfo,
	}, nil
}

// context bootstraps (once per process) the build id and authURL by loading a
// member page with the session cookie.
func (c *Client) context() (*shaktiContext, error) {
	if c.ctx != nil {
		return c.ctx, nil
	}
	if c.Cookie == "" {
		return nil, ErrNoSession
	}
	html, err := c.GetText(c.BaseURL + "/browse")
	if err != nil {
		return nil, err
	}
	ctx, err := parseReactContext(html)
	if err != nil {
		return nil, err
	}
	c.ctx = ctx
	return ctx, nil
}

// Whoami returns the account summary carried by the current session.
func (c *Client) Whoami() (UserInfo, error) {
	ctx, err := c.context()
	if err != nil {
		return UserInfo{}, err
	}
	if ctx.User.MembershipStatus == "" || ctx.User.MembershipStatus == "ANONYMOUS" {
		return ctx.User, fmt.Errorf("the session is not signed in (membership status %q) — re-import cookies with `netflix login --from-browser chrome`", ctx.User.MembershipStatus)
	}
	return ctx.User, nil
}

// PathEvaluator issues a Falcor query against Shakti, the same API the web app
// uses. Each path is a Falcor path, e.g.
// ["profilesList", {"from":0,"to":4}, ["summary"]].
func (c *Client) PathEvaluator(paths ...any) (json.RawMessage, error) {
	ctx, err := c.context()
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	for _, p := range paths {
		raw, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("encode falcor path: %w", err)
		}
		form.Add("path", string(raw))
	}
	form.Set("authURL", ctx.AuthURL)
	endpoint := fmt.Sprintf("%s/api/shakti/%s/pathEvaluator?method=get&falcor_server=0.1.0", c.BaseURL, url.PathEscape(ctx.BuildID))
	if c.Profile != "" {
		endpoint += "&profileGuid=" + url.QueryEscape(c.Profile)
	}
	req, err := c.newReq("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json, text/javascript, */*")
	req.Header.Set("x-netflix.client.request.name", "ui/falcorUnclassified")
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	data, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// GetJSON fetches a Shakti REST endpoint (viewingactivity, billingActivity, …)
// relative to /api/shakti/<build>/, decoding the response into v.
func (c *Client) GetJSON(path string, query url.Values, v any) error {
	ctx, err := c.context()
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/api/shakti/%s/%s", c.BaseURL, url.PathEscape(ctx.BuildID), strings.TrimPrefix(path, "/"))
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := c.newReq("GET", endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json, text/javascript, */*")
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	data, err := c.do(req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
