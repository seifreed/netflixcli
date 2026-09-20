package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These exercise whole commands — dispatch, flags, the client, and rendering —
// against a stub standing in for netflix.com. Only the surfaces Netflix
// server-renders are covered here, because those need nothing but the page:
// the rest go to the GraphQL gateway, which is tested in the client package.

// netflixPage is a Netflix member page in miniature: the bootstrap script
// carrying the build and the account, then the Apollo cache carrying the
// profiles and the rows. Both use the escaping the real pages use — \xNN in the
// bootstrap, a single-quoted JavaScript literal around the cache.
const netflixPage = `<!DOCTYPE html><html><body><script>` +
	`netflix.reactContext = {"models":{"serverDefs":{"data":{"BUILD_IDENTIFIER":"v1a09dd61","originalUrl":"\x2Fbrowse"}},` +
	`"userInfo":{"data":{"name":"Ada","accountOwnerName":"Ada","membershipStatus":"CURRENT_MEMBER",` +
	`"countryOfSignup":"ES","memberSince":"junio de 2019","numProfiles":2}}}};</script>` +
	`<script>netflix.reactContext.models.graphql = JSON.parse('{"data":{` +
	`"ROOT_QUERY":{"account":{"__ref":"Account:1"},"currentProfile":{"__ref":"Profile:A"},` +
	`"pinotBrowsePage({\\"categoryId\\":null})":{"__ref":"Page:1"}},` +
	`"Account:1":{"profiles":[{"__ref":"Profile:A"},{"__ref":"Profile:B"}]},` +
	`"Profile:A":{"guid":"AAA","name":"Ada","isKids":false,"isPinLocked":false},` +
	`"Profile:B":{"guid":"BBB","name":"Kid","isKids":true,"isPinLocked":false},` +
	`"Page:1":{"sections({\\"first\\":8})":{"edges":[{"node":{"__ref":"Section:1"}},{"node":{"__ref":"Section:2"}}]}},` +
	`"Section:1":{"__typename":"PinotCarouselSection","displayString":"Mi lista",` +
	`"eventListeners":[{"actions":[{"__ref":"PinotPageUpdateAction:CghwbGF5bGlzdBICCDc="}]}],` +
	`"entities":{"edges":[{"node":{"__ref":"Card:1"}}]}},` +
	`"Section:2":{"__typename":"PinotCarouselSection","displayString":"Tendencias",` +
	`"entities":{"edges":[{"node":{"__ref":"Card:2"}}]}},` +
	`"Card:1":{"displayString":"Dark","unifiedEntity":{"__ref":"Show:1"}},` +
	`"Card:2":{"displayString":"Fariña","unifiedEntity":{"__ref":"Show:2"}},` +
	`"Show:1":{"__typename":"Show","videoId":80100172},` +
	`"Show:2":{"__typename":"Show","videoId":80215500}` +
	`}}');</script></body></html>`

// stubNetflix points the CLI at a server that answers every page with one
// fixture, and gives it a throwaway config directory holding a session.
func stubNetflix(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "text/html")
		if _, err := w.Write([]byte(netflixPage)); err != nil {
			t.Errorf("stub write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	t.Setenv("NETFLIX_CONFIG_DIR", dir)
	t.Setenv("NETFLIX_BASE_URL", srv.URL)
	session := `{"cookie":"NetflixId=stub; SecureNetflixId=stub"}`
	if err := os.WriteFile(filepath.Join(dir, "session.json"), []byte(session), 0o600); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	return srv
}

// runCommand runs the CLI as main would and returns its exit code and stdout.
func runCommand(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var code int
	out := captureStdout(t, func() { code = run(args) })
	return code, out
}

func TestWhoamiReportsTheAccountAndProfile(t *testing.T) {
	stubNetflix(t)
	code, out := runCommand(t, "whoami")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	for _, want := range []string{"Ada (CURRENT_MEMBER)", "country: ES", "acting as: Ada (AAA)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q is missing %q", out, want)
		}
	}
}

func TestWhoamiJSONCarriesTheSessionState(t *testing.T) {
	stubNetflix(t)
	code, out := runCommand(t, "whoami", "--json")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	var got struct {
		HasCookie bool `json:"hasCookie"`
		Accepted  bool `json:"accepted"`
		User      struct {
			Name string `json:"name"`
		} `json:"user"`
		Profile struct {
			GUID string `json:"guid"`
		} `json:"profile"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("--json did not emit JSON (%v): %q", err, out)
	}
	if !got.HasCookie || !got.Accepted || got.User.Name != "Ada" || got.Profile.GUID != "AAA" {
		t.Errorf("decoded %+v, want the stub's account and active profile", got)
	}
}

func TestProfilesMarksTheActiveOne(t *testing.T) {
	stubNetflix(t)
	code, out := runCommand(t, "profiles")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "* Ada") {
		t.Errorf("output %q does not mark Ada as active", out)
	}
	if !strings.Contains(out, "Kid") || !strings.Contains(out, "(kids)") {
		t.Errorf("output %q does not list the kids profile", out)
	}
}

func TestBrowseRendersEveryRow(t *testing.T) {
	stubNetflix(t)
	code, out := runCommand(t, "browse")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	for _, want := range []string{"Mi lista", "Dark", "Tendencias", "Fariña"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q is missing %q", out, want)
		}
	}
}

// The feed is found by the id Netflix encodes in the row's actions, not by the
// localised row title.
func TestMyListReadsThePersonalRow(t *testing.T) {
	stubNetflix(t)
	code, out := runCommand(t, "mylist")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "Dark") {
		t.Errorf("output %q does not list the My List title", out)
	}
	if strings.Contains(out, "Fariña") {
		t.Error("mylist leaked a title from another row")
	}
}

func TestLimitCapsTheRow(t *testing.T) {
	stubNetflix(t)
	_, out := runCommand(t, "browse", "--limit", "0")
	if !strings.Contains(out, "Dark") {
		t.Fatalf("a zero limit dropped titles: %q", out)
	}
}

// A command whose feed the page does not carry must say so, and must fail.
func TestMissingFeedFails(t *testing.T) {
	stubNetflix(t)
	code, _ := runCommand(t, "liked")
	if code != exitError {
		t.Errorf("exit code = %d, want %d when the row is absent", code, exitError)
	}
}

func TestCommandWithoutASessionFails(t *testing.T) {
	stubNetflix(t)
	// A config directory with no session in it at all.
	t.Setenv("NETFLIX_CONFIG_DIR", t.TempDir())
	code, _ := runCommand(t, "whoami")
	if code != exitError {
		t.Errorf("exit code = %d, want %d without a session", code, exitError)
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	code, _ := runCommand(t, "nonsense")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}
