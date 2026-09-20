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

// The rest of the commands go to the GraphQL gateway. Pointing NETFLIX_GRAPHQL_URL
// at a stub and seeding the query map — which is what a real run caches after
// its first call — lets those run end to end too.
func stubGateway(t *testing.T, responses map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		op := r.Header.Get("x-netflix.context.operation-name")
		body, ok := responses[op]
		if !ok {
			t.Errorf("stub gateway got an unexpected operation %q", op)
			http.Error(w, "unexpected operation", http.StatusNotImplemented)
			return
		}
		w.Header().Set("content-type", "application/json")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("stub gateway write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("NETFLIX_GRAPHQL_URL", srv.URL)

	manifest := `{"build":"v1a09dd61","version":102,"ops":{` +
		`"SearchPageQueryResults":"11111111-1111-1111-1111-111111111111",` +
		`"GetGenreSubgenres":"22222222-2222-2222-2222-222222222222",` +
		`"DetailModal":"33333333-3333-3333-3333-333333333333"}}`
	path := filepath.Join(os.Getenv("NETFLIX_CONFIG_DIR"), "queries.json")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatalf("seed query manifest: %v", err)
	}
}

func TestSearchRendersResults(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"SearchPageQueryResults": `{"data":{"page":{"sections":{"edges":[
			{"node":{"__typename":"PinotGallerySection","_id":"g1","entities":{
				"pageInfo":{"hasNextPage":false},
				"edges":[{"node":{"displayString":"Dark","unifiedEntity":{"__typename":"Show","videoId":80100172}}}]}}}]}}}}`,
	})
	code, out := runCommand(t, "search", "dark")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "Dark") || !strings.Contains(out, "80100172") {
		t.Errorf("output %q does not carry the result", out)
	}
}

func TestSearchWithoutResultsSaysSo(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"SearchPageQueryResults": `{"data":{"page":{"sections":{"edges":[]}}}}`,
	})
	code, out := runCommand(t, "search", "nothingmatchesthis")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "no results") {
		t.Errorf("output %q does not report an empty search", out)
	}
}

// A GraphQL error arrives inside a 200; it must still fail the command.
func TestGraphQLErrorFailsTheCommand(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"SearchPageQueryResults": `{"errors":[{"message":"nope"}],"data":null}`,
	})
	code, _ := runCommand(t, "search", "dark")
	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
}

func TestGenresListsTheRegionMenu(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"GetGenreSubgenres": `{"data":{"navigationMenuCategories":[
			{"id":"genre-8711","title":"Películas de terror"},
			{"id":"genre-6548","title":"Comedias"}]}}`,
	})
	code, out := runCommand(t, "genres", "terror")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(out, "8711") {
		t.Errorf("output %q does not carry the matching genre id", out)
	}
	if strings.Contains(out, "Comedias") {
		t.Errorf("output %q was not filtered", out)
	}
}

func TestTitleRendersTheCertificationNotTheMaturityNumber(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"DetailModal": `{"data":{"unifiedEntities":[{"__typename":"Show","videoId":80100172,
			"title":"Dark","latestYear":2020,"runtimeSec":3060,
			"contentAdvisory":{"certificationValue":"16+","maturityLevel":90},
			"contextualSynopsis":{"text":"Un pueblo alemán."},
			"genreTags":{"edges":[{"node":{"name":"Series de misterio"}}]},
			"cast":{"edges":[{"node":{"name":"Louis Hofmann"}}]}}]}}`,
	})
	code, out := runCommand(t, "title", "80100172")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	for _, want := range []string{"Dark", "2020", "51m", "16+", "Un pueblo alemán.", "Louis Hofmann"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q is missing %q", out, want)
		}
	}
	if strings.Contains(out, "90") {
		t.Errorf("output %q leaks Netflix's internal maturity number", out)
	}
}

func TestTitleRejectsAnOperandThatIsNotAnID(t *testing.T) {
	stubNetflix(t)
	code, _ := runCommand(t, "title", "not-an-id")
	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
}
