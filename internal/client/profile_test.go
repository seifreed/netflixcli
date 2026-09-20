package client

import (
	"net/http"
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
