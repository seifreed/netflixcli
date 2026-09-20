package client

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/seifreed/netflixcli/internal/cookie"
)

// Profile is one profile of the account the session belongs to.
type Profile struct {
	GUID        string `json:"guid"`
	Name        string `json:"name"`
	IsKids      bool   `json:"isKids"`
	IsPinLocked bool   `json:"isPinLocked"`
	Current     bool   `json:"current"`
}

// CurrentProfile reports the profile the session is acting as. It comes from
// the page bootstrap, so it is free once anything else has run.
func (c *Client) CurrentProfile() (Profile, error) {
	ctx, err := c.context()
	if err != nil {
		return Profile{}, err
	}
	if ctx.Profile.GUID == "" {
		return Profile{}, fmt.Errorf("netflix did not say which profile this session is using")
	}
	return ctx.Profile, nil
}

// Profiles lists the account's profiles, marking the one the session is
// currently acting as. They are read out of the page bootstrap, so this costs
// one request.
func (c *Client) Profiles() ([]Profile, error) {
	if c.Cookie == "" {
		return nil, ErrNoSession
	}
	html, err := c.GetText(c.BaseURL + "/browse")
	if err != nil {
		return nil, err
	}
	cache, err := parseApolloCache(html)
	if err != nil {
		return nil, err
	}
	return cache.profiles(), nil
}

// currentProfile is the profile the session is acting as.
func (c apolloCache) currentProfile() Profile {
	entity := c.deref(field(c["ROOT_QUERY"], "currentProfile"))
	isKids, _ := field(entity, "isKids").(bool)
	return Profile{
		GUID:    fieldString(entity, "guid"),
		Name:    strings.TrimSpace(fieldString(entity, "name")),
		IsKids:  isKids,
		Current: true,
	}
}

func (c apolloCache) profiles() []Profile {
	current := c.currentProfile().GUID
	refs, _ := field(c.deref(field(c["ROOT_QUERY"], "account")), "profiles").([]any)
	profiles := make([]Profile, 0, len(refs))
	for _, ref := range refs {
		entity := c.deref(ref)
		guid := fieldString(entity, "guid")
		if guid == "" {
			continue
		}
		isKids, _ := field(entity, "isKids").(bool)
		isPinLocked, _ := field(entity, "isPinLocked").(bool)
		profiles = append(profiles, Profile{
			GUID:        guid,
			Name:        strings.TrimSpace(fieldString(entity, "name")),
			IsKids:      isKids,
			IsPinLocked: isPinLocked,
			Current:     guid == current,
		})
	}
	sort.SliceStable(profiles, func(i, j int) bool { return profiles[i].Current && !profiles[j].Current })
	return profiles
}

// ResolveProfile finds a profile by guid or by (case-insensitive) name.
func (c *Client) ResolveProfile(nameOrGUID string) (Profile, error) {
	want := strings.TrimSpace(nameOrGUID)
	if want == "" {
		return Profile{}, fmt.Errorf("no profile given")
	}
	profiles, err := c.Profiles()
	if err != nil {
		return Profile{}, err
	}
	for _, p := range profiles {
		if p.GUID == want || strings.EqualFold(p.Name, want) {
			return p, nil
		}
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	return Profile{}, fmt.Errorf("no profile named %q on this account (have: %s)", want, strings.Join(names, ", "))
}

// UseProfile re-points the session at another profile the way the web app's
// profile switcher does, and returns the refreshed Cookie header. A PIN-locked
// profile cannot be entered this way: Netflix asks for the PIN in the browser.
func (c *Client) UseProfile(nameOrGUID string) (Profile, string, error) {
	profile, err := c.ResolveProfile(nameOrGUID)
	if err != nil {
		return Profile{}, "", err
	}
	if profile.Current {
		return profile, c.Cookie, nil
	}
	if profile.IsPinLocked {
		return profile, "", fmt.Errorf("profile %q is PIN-locked — unlock it in the browser, then re-import the session", profile.Name)
	}
	switchURL := fmt.Sprintf("%s/SwitchProfile?tkn=%s", c.BaseURL, url.QueryEscape(profile.GUID))
	req, err := c.newReq("GET", switchURL, nil)
	if err != nil {
		return profile, "", err
	}
	resp, err := c.noRedirectDo(req)
	if err != nil {
		return profile, "", err
	}
	defer resp.Body.Close()
	updated := cookie.Merge(c.Cookie, resp.Cookies())
	if updated == c.Cookie {
		return profile, "", fmt.Errorf("netflix did not switch to %q (HTTP %d)", profile.Name, resp.StatusCode)
	}
	c.Cookie = updated
	c.ctx = nil // the bootstrap belongs to the profile that issued it
	return profile, updated, nil
}

// noRedirectDo sends req without following redirects, so the Set-Cookie headers
// of a 302 are visible to the caller.
func (c *Client) noRedirectDo(req *http.Request) (*http.Response, error) {
	client := *c.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client.Do(req)
}
