package client

import (
	"net/http"
	"strings"
	"testing"
)

const pageWithProfiles = `<script>netflix.reactContext.models.graphql = JSON.parse('{"data":{` +
	`"ROOT_QUERY":{"account":{"__ref":"Account:1"},"currentProfile":{"__ref":"Profile:{\\"guid\\":\\"BBB\\"}"}},` +
	`"Account:1":{"profiles":[{"__ref":"Profile:{\\"guid\\":\\"AAA\\"}"},{"__ref":"Profile:{\\"guid\\":\\"BBB\\"}"},{"__ref":"Profile:{\\"guid\\":\\"CCC\\"}"}]},` +
	`"Profile:{\\"guid\\":\\"AAA\\"}":{"guid":"AAA","name":"Ada","isKids":false,"isPinLocked":true},` +
	`"Profile:{\\"guid\\":\\"BBB\\"}":{"guid":"BBB","name":"Grace ","isKids":false,"isPinLocked":false},` +
	`"Profile:{\\"guid\\":\\"CCC\\"}":{"guid":"CCC","name":"Kid","isKids":true,"isPinLocked":false}` +
	`}}');</script>`

func TestApolloCacheProfiles(t *testing.T) {
	cache, err := parseApolloCache(pageWithProfiles)
	if err != nil {
		t.Fatalf("parseApolloCache: %v", err)
	}
	profiles := cache.profiles()
	if len(profiles) != 3 {
		t.Fatalf("got %d profiles, want 3", len(profiles))
	}
	if !profiles[0].Current || profiles[0].GUID != "BBB" {
		t.Errorf("first profile = %+v, want the current one (BBB) hoisted", profiles[0])
	}
	if profiles[0].Name != "Grace" {
		t.Errorf("name = %q, want the trailing space trimmed", profiles[0].Name)
	}
	var kid, locked bool
	for _, p := range profiles {
		if p.GUID == "CCC" {
			kid = p.IsKids
		}
		if p.GUID == "AAA" {
			locked = p.IsPinLocked
		}
	}
	if !kid || !locked {
		t.Errorf("kids=%v pinLocked=%v, want both true", kid, locked)
	}
}

func TestApolloCacheCurrentProfile(t *testing.T) {
	cache, err := parseApolloCache(pageWithProfiles)
	if err != nil {
		t.Fatalf("parseApolloCache: %v", err)
	}
	got := cache.currentProfile()
	if got.GUID != "BBB" || got.Name != "Grace" || !got.Current {
		t.Errorf("currentProfile = %+v, want the trimmed BBB marked current", got)
	}
}

// The account guid in the page bootstrap is not the profile guid; anything that
// asks "which profile?" has to read the profile cache instead.
func TestCurrentProfileIsEmptyWithoutACache(t *testing.T) {
	var cache apolloCache
	if got := cache.currentProfile(); got.GUID != "" {
		t.Errorf("currentProfile = %+v, want the zero value", got)
	}
}

// memberPage carries both halves the client bootstraps from: the reactContext
// blob (see shakti_test.go) and the Apollo cache holding the profiles.
const memberPage = bootstrapPage + pageWithProfiles

// Who the session is, who it acts as and what else it could act as all come off
// the one page the client bootstraps from. Profiles used to fetch it again, so
// a configured default profile cost two loads before any command ran.
func TestTheAccountIsReadFromOnePageLoad(t *testing.T) {
	var loads int
	c, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		loads++
		if _, err := w.Write([]byte(memberPage)); err != nil {
			t.Errorf("stub write: %v", err)
		}
	})
	c.Cookie = "NetflixId=stub"

	if _, err := c.Account.Whoami(); err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	profiles, err := c.Account.Profiles()
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if _, err := c.Account.CurrentProfile(); err != nil {
		t.Fatalf("CurrentProfile: %v", err)
	}
	if len(profiles) != 3 {
		t.Errorf("got %d profiles, want 3", len(profiles))
	}
	if loads != 1 {
		t.Errorf("the page was loaded %d times, want 1", loads)
	}
}

// A profile switch invalidates the bootstrap, so the list is read afresh.
func TestSwitchingProfilesRereadsTheAccount(t *testing.T) {
	var loads int
	c, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		loads++
		if _, err := w.Write([]byte(memberPage)); err != nil {
			t.Errorf("stub write: %v", err)
		}
	})
	c.Cookie = "NetflixId=stub"

	if _, err := c.Account.Profiles(); err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	c.ctx = nil // what UseProfile does once netflix has accepted the switch
	if _, err := c.Account.Profiles(); err != nil {
		t.Fatalf("Profiles after switch: %v", err)
	}
	if loads != 2 {
		t.Errorf("the page was loaded %d times, want 2", loads)
	}
}

// switchStub serves the member page everywhere, and answers /SwitchProfile the
// way Netflix does: a redirect whose Set-Cookie headers carry the new session.
func switchStub(t *testing.T, setCookies []string) (*Client, *int) {
	t.Helper()
	switches := 0
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/SwitchProfile") {
			switches++
			for _, c := range setCookies {
				w.Header().Add("Set-Cookie", c)
			}
			w.WriteHeader(http.StatusFound)
			return
		}
		if _, err := w.Write([]byte(memberPage)); err != nil {
			t.Errorf("stub write: %v", err)
		}
	})
	c.Cookie = "NetflixId=old; nfvdid=keep"
	return c, &switches
}

func TestUseProfileSwitchesAndRefreshesTheSession(t *testing.T) {
	c, switches := switchStub(t, []string{"NetflixId=new; Path=/", "extra=1; Path=/"})

	profile, updated, err := c.Account.UseProfile("kid")
	if err != nil {
		t.Fatalf("UseProfile: %v", err)
	}
	if *switches != 1 {
		t.Errorf("SwitchProfile was called %d times, want 1", *switches)
	}
	if profile.GUID != "CCC" {
		t.Errorf("switched to %q, want CCC — the name match is case-insensitive", profile.GUID)
	}
	for _, want := range []string{"NetflixId=new", "nfvdid=keep", "extra=1"} {
		if !strings.Contains(updated, want) {
			t.Errorf("refreshed cookie %q is missing %q", updated, want)
		}
	}
	if c.Cookie != updated {
		t.Error("the client kept the old cookie")
	}
	if c.ctx != nil {
		t.Error("the bootstrap survived the switch; it belongs to the profile that issued it")
	}
}

// The profile the session already acts as needs no request, and must not look
// like a failed switch.
func TestUseProfileOnTheCurrentProfileIsANoOp(t *testing.T) {
	c, switches := switchStub(t, nil)
	before := c.Cookie

	profile, updated, err := c.Account.UseProfile("Grace")
	if err != nil {
		t.Fatalf("UseProfile: %v", err)
	}
	if !profile.Current || updated != before {
		t.Errorf("profile=%+v cookie=%q, want the current profile and an untouched cookie", profile, updated)
	}
	if *switches != 0 {
		t.Errorf("SwitchProfile was called %d times for a profile already in use", *switches)
	}
}

func TestUseProfileRefusesAPinLockedProfile(t *testing.T) {
	c, switches := switchStub(t, nil)

	if _, _, err := c.Account.UseProfile("Ada"); err == nil {
		t.Fatal("want an error for a PIN-locked profile")
	} else if !strings.Contains(err.Error(), "PIN-locked") {
		t.Errorf("error %q does not say the profile is PIN-locked", err)
	}
	if *switches != 0 {
		t.Errorf("SwitchProfile was called %d times for a PIN-locked profile", *switches)
	}
}

// Netflix answers a refused switch with a redirect that changes no cookie. The
// session must not be reported as switched.
func TestUseProfileReportsARefusedSwitch(t *testing.T) {
	c, _ := switchStub(t, nil)
	before := c.Cookie

	if _, _, err := c.Account.UseProfile("Kid"); err == nil {
		t.Fatal("want an error when netflix changes no cookie")
	} else if !strings.Contains(err.Error(), "did not switch") {
		t.Errorf("error %q does not say the switch was refused", err)
	}
	if c.Cookie != before {
		t.Error("a refused switch changed the session cookie")
	}
}

func TestResolveProfileNamesTheOnesItHas(t *testing.T) {
	c, _ := switchStub(t, nil)

	if _, err := c.Account.ResolveProfile("  "); err == nil {
		t.Error("want an error for a blank profile")
	}
	_, err := c.Account.ResolveProfile("Nobody")
	if err == nil {
		t.Fatal("want an error for a profile that does not exist")
	}
	for _, name := range []string{"Ada", "Grace", "Kid"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not list %q as an option", err, name)
		}
	}
	byGUID, err := c.Account.ResolveProfile("AAA")
	if err != nil || byGUID.Name != "Ada" {
		t.Errorf("ResolveProfile by guid = %+v, %v", byGUID, err)
	}
}
