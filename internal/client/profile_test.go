package client

import "testing"

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
