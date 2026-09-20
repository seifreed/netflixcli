package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	`"Page:1":{"id":"PS_stub_L1_N1","sections({\\"first\\":8})":{"edges":[{"node":{"__ref":"Section:1"}},{"node":{"__ref":"Section:2"}}]}},` +
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
func stubNetflix(t *testing.T) {
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
// playlistActionID is the base64 page-update id Netflix puts on the My List
// row; it is what names the row, since the title is localised.
const playlistActionID = "CghwbGF5bGlzdBICCDc="

// feedSection renders one My Netflix row as the gateway sends it.
func feedSection(action, name string, total int, titles ...string) string {
	cards := make([]string, 0, len(titles))
	for i, title := range titles {
		cards = append(cards, fmt.Sprintf(
			`{"node":{"displayString":%q,"unifiedEntity":{"__typename":"Show","videoId":%d}}}`,
			title, 80100000+i))
	}
	listeners := ""
	if action != "" {
		listeners = fmt.Sprintf(`"eventListeners":[{"actions":[{"id":%q}]}],`, action)
	}
	return fmt.Sprintf(`{"node":{"__typename":"PinotCarouselSection","_id":%q,"displayString":%q,%s
		"entities":{"totalCount":%d,"pageInfo":{"endCursor":"c","hasNextPage":%t},"edges":[%s]}}}`,
		name, name, listeners, total, len(titles) < total, strings.Join(cards, ","))
}

func sectionsResponse(sections ...string) string {
	return `{"data":{"page":{"sections":{"pageInfo":{"endCursor":"","hasNextPage":false},"edges":[` +
		strings.Join(sections, ",") + `]}}}}`
}

func TestMyListReadsThePersonalRow(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"FetchMoreSections": sectionsResponse(
			feedSection(playlistActionID, "Mi lista", 1, "Dark"),
			feedSection("", "Tendencias", 1, "Fariña"),
		),
	})
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

// The carousel in the page carries thirteen titles whatever the list holds, so
// `mylist` used to answer with thirteen of three hundred and fifty-two. The
// reply says how long the list is; a short one must be asked for again.
func TestMyListAsksAgainWhenTheListIsLongerThanTheReply(t *testing.T) {
	stubNetflix(t)
	// A list longer than the first ask, the way a real My List is.
	const inTheList = 150
	all := make([]string, inTheList)
	for i := range all {
		all[i] = fmt.Sprintf("Title %d", i+1)
	}
	var sizes []int
	stubVaryingGateway(t, func(op string, vars map[string]any) string {
		if op != "FetchMoreSections" {
			t.Errorf("unexpected operation %q", op)
			return "{}"
		}
		size := int(vars["carouselPageSize"].(float64))
		sizes = append(sizes, size)
		served := all
		if size < len(served) {
			served = served[:size]
		}
		return sectionsResponse(feedSection(playlistActionID, "Mi lista", inTheList, served...))
	})

	code, out := runCommand(t, "mylist", "--json")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	var titles []map[string]any
	if err := json.Unmarshal([]byte(out), &titles); err != nil {
		t.Fatalf("decode: %v (%q)", err, out)
	}
	if len(titles) != inTheList {
		t.Errorf("got %d titles, want the whole list of %d", len(titles), inTheList)
	}
	if len(sizes) != 2 || sizes[1] != inTheList {
		t.Errorf("asked with carousel sizes %v, want a second ask for all %d", sizes, inTheList)
	}
}

// --limit must not make it ask for more than the caller wants.
func TestMyListLimitStopsItAskingForTheWholeList(t *testing.T) {
	stubNetflix(t)
	var sizes []int
	stubVaryingGateway(t, func(_ string, vars map[string]any) string {
		size := int(vars["carouselPageSize"].(float64))
		sizes = append(sizes, size)
		return sectionsResponse(feedSection(playlistActionID, "Mi lista", 352, "Dark", "Fariña"))
	})

	if code, _ := runCommand(t, "mylist", "--limit", "2"); code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	if len(sizes) != 1 || sizes[0] != 2 {
		t.Errorf("asked with carousel sizes %v, want one ask for 2", sizes)
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
// stubVaryingGateway answers each gateway call from the operation and the
// variables it carries, for the tests where the second request must differ from
// the first.
func stubVaryingGateway(t *testing.T, answer func(op string, vars map[string]any) string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("content-type", "application/json")
		if _, err := w.Write([]byte(answer(r.Header.Get("x-netflix.context.operation-name"), body.Variables))); err != nil {
			t.Errorf("stub gateway write: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("NETFLIX_GRAPHQL_URL", srv.URL)

	seedQueryManifest(t)
}

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

	seedQueryManifest(t)
}

// seedQueryManifest writes the persisted-query map a real run would scrape from
// the client bundle, so the stubs are reached without downloading one.
func seedQueryManifest(t *testing.T) {
	t.Helper()
	manifest := `{"build":"v1a09dd61","version":102,"ops":` + stubQueryIDs + `}`
	path := filepath.Join(os.Getenv("NETFLIX_CONFIG_DIR"), "queries.json")
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatalf("seed query manifest: %v", err)
	}
}

// stubQueryIDs stands in for the map a real run scrapes from the client bundle
// and caches; the ids themselves are never checked by the stub gateway.
const stubQueryIDs = `{
	"SearchPageQueryResults":"11111111-1111-1111-1111-111111111111",
	"GetGenreSubgenres":"22222222-2222-2222-2222-222222222222",
	"DetailModal":"33333333-3333-3333-3333-333333333333",
	"PreviewModalEpisodeSelector":"44444444-4444-4444-4444-444444444444",
	"PreviewModalEpisodeSelectorSeasonEpisodes":"55555555-5555-5555-5555-555555555555",
	"AddToPlaylist":"66666666-6666-6666-6666-666666666666",
	"RemoveFromPlaylist":"77777777-7777-7777-7777-777777777777",
	"SetEntityThumbRating":"88888888-8888-8888-8888-888888888888",
	"RemoveFromContinueWatching":"99999999-9999-9999-9999-999999999999",
	"AddReminder":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	"FetchMoreSections":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
}`

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

func TestSeasonsAndEpisodes(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"PreviewModalEpisodeSelector": `{"data":{"videos":[{"__typename":"Show","videoId":80100172,
			"seasons":{"edges":[{"node":{"videoId":80114789,"number":1,"title":"Temporada 1",
				"episodes":{"totalCount":10},"contentAdvisory":{"certificationValue":"16+"}}}]}}]}}`,
		"PreviewModalEpisodeSelectorSeasonEpisodes": `{"data":{"videos":[{"episodes":{"edges":[
			{"node":{"videoId":80114790,"number":1,"title":"Secretos","runtimeSec":3091,
				"isPlayable":true,"contextualSynopsis":{"text":"Un pueblo alemán."}}}]}}]}}`,
	})
	code, out := runCommand(t, "seasons", "80100172")
	if code != exitOK {
		t.Fatalf("seasons exit code = %d", code)
	}
	if !strings.Contains(out, "Temporada 1") || !strings.Contains(out, "10 episodes") {
		t.Errorf("seasons output %q is missing the season", out)
	}

	code, out = runCommand(t, "episodes", "80100172")
	if code != exitOK {
		t.Fatalf("episodes exit code = %d", code)
	}
	for _, want := range []string{"Secretos", "51m", "Un pueblo alemán."} {
		if !strings.Contains(out, want) {
			t.Errorf("episodes output %q is missing %q", out, want)
		}
	}
}

// Asking for episodes of a movie must say so rather than print an empty list.
func TestEpisodesOfAMovieExplainsItself(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"PreviewModalEpisodeSelector": `{"data":{"videos":[{"__typename":"Movie","videoId":70095139,"seasons":{"edges":[]}}]}}`,
	})
	code, _ := runCommand(t, "episodes", "70095139")
	if code != exitError {
		t.Errorf("exit code = %d, want %d for a movie", code, exitError)
	}
}

func TestMyListAddAndRemoveReportTheResultingState(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"AddToPlaylist": `{"data":{"addEntityToPlaylist":{"entity":
			{"videoId":80100172,"title":"Dark","isInPlaylist":true}}}}`,
		"RemoveFromPlaylist": `{"data":{"removeEntityFromPlaylist":{"entity":
			{"videoId":80100172,"title":"Dark","isInPlaylist":false}}}}`,
	})
	code, out := runCommand(t, "mylist", "add", "80100172")
	if code != exitOK || !strings.Contains(out, "Dark is in My List") {
		t.Errorf("add: code=%d out=%q", code, out)
	}
	code, out = runCommand(t, "mylist", "remove", "80100172")
	if code != exitOK || !strings.Contains(out, "no longer in My List") {
		t.Errorf("remove: code=%d out=%q", code, out)
	}
}

// Netflix reports some refusals inside a 200 payload; a write must not report
// success when that happens.
func TestARefusedWriteFails(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"SetEntityThumbRating": `{"data":{"setEntityThumbRating":{"entity":{},"errors":[{"message":"not allowed"}]}}}`,
	})
	code, out := runCommand(t, "rate", "80100172", "up")
	if code != exitError {
		t.Errorf("exit code = %d, want %d when Netflix refuses", code, exitError)
	}
	if strings.Contains(out, "thumbs up") {
		t.Errorf("output %q claims the rating was set", out)
	}
}

func TestRateRejectsAnUnknownRating(t *testing.T) {
	stubNetflix(t)
	code, _ := runCommand(t, "rate", "80100172", "sideways")
	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
}

func TestContinueRemove(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"RemoveFromContinueWatching": `{"data":{"removeFromContinueWatching":{"success":true}}}`,
	})
	code, out := runCommand(t, "continue", "remove", "80100172")
	if code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out, "no longer in Continue Watching") {
		t.Errorf("output %q does not confirm the removal", out)
	}
}

func TestContinueRemoveReportsAFailedRemoval(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"RemoveFromContinueWatching": `{"data":{"removeFromContinueWatching":{"success":false}}}`,
	})
	code, _ := runCommand(t, "continue", "remove", "80100172")
	if code != exitError {
		t.Errorf("exit code = %d, want %d when Netflix reports no success", code, exitError)
	}
}

// A reminder on an already-available title lands in My List instead; the
// command must report both flags rather than claim a reminder was set.
func TestRemindReportsBothFlags(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"AddReminder": `{"data":{"addUnifiedEntityToRemindMe":
			{"videoId":80100172,"isInRemindMeList":true,"isInPlaylist":true}}}`,
	})
	code, out := runCommand(t, "remind", "add", "80100172")
	if code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out, "reminder: yes") || !strings.Contains(out, "My List: yes") {
		t.Errorf("output %q does not report both flags", out)
	}
	if strings.Contains(out, "will remind you") {
		t.Errorf("output %q narrates a reminder it cannot vouch for", out)
	}
}

// A bare parent is a wrong invocation, not a failed command. What it prints is
// asserted in TestParentCommandsNameTheirSubcommands, since it goes to stderr.
func TestRemindNeedsASubcommand(t *testing.T) {
	stubNetflix(t)
	code, _ := runCommand(t, "remind", "80100172")
	if code != exitUsage {
		t.Errorf("exit code = %d, want %d without add|remove", code, exitUsage)
	}
}

// The session commands write the file every later command reads, so what lands
// on disk matters as much as what is printed.
func TestSetCookieStoresTheSession(t *testing.T) {
	stubNetflix(t)
	code, _ := runCommand(t, "set-cookie", "NetflixId=fresh; nfvdid=x")
	if code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	stored := readSession(t)
	if !strings.Contains(stored, "NetflixId=fresh") {
		t.Errorf("stored session %q does not carry the cookie", stored)
	}
}

func TestSetCookieRejectsAnEmptyCookie(t *testing.T) {
	stubNetflix(t)
	code, _ := runCommand(t, "set-cookie")
	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
}

func TestImportHarTakesTheSignedInRequest(t *testing.T) {
	stubNetflix(t)
	har := `{"log":{"entries":[
		{"request":{"url":"https://evil.example/","headers":[{"name":"Cookie","value":"NetflixId=stolen"}]}},
		{"request":{"url":"https://www.netflix.com/browse","headers":[{"name":"Cookie","value":"NetflixId=mine"}]}}
	]}}`
	path := filepath.Join(t.TempDir(), "netflix.har")
	if err := os.WriteFile(path, []byte(har), 0o600); err != nil {
		t.Fatalf("write har: %v", err)
	}
	code, _ := runCommand(t, "import-har", "--file", path)
	if code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	stored := readSession(t)
	if !strings.Contains(stored, "NetflixId=mine") {
		t.Errorf("stored session %q did not take the Netflix request's cookie", stored)
	}
	if strings.Contains(stored, "stolen") {
		t.Error("import-har took a cookie belonging to another site")
	}
}

func TestImportHarNeedsAFile(t *testing.T) {
	stubNetflix(t)
	code, _ := runCommand(t, "import-har")
	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
}

func readSession(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(os.Getenv("NETFLIX_CONFIG_DIR"), "session.json"))
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	return string(raw)
}

// The top-10 rows are not in the page: they arrive over the section cursor, and
// the command numbers them because the order is the answer.
func TestTopNumbersTheRankedRows(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"FetchMoreSections": `{"data":{"page":{"sections":{
			"pageInfo":{"endCursor":"","hasNextPage":false},
			"edges":[
				{"node":{"displayString":"Series dramáticas","entities":{"edges":[
					{"node":{"__typename":"PinotStandardBoxshotEntityTreatment","displayString":"Dark",
						"unifiedEntity":{"__typename":"Show","videoId":80100172}}}]}}},
				{"node":{"displayString":"Las 10 series más populares","entities":{"edges":[
					{"node":{"__typename":"PinotRankedBoxshotEntityTreatment","displayString":"Monstruo",
						"unifiedEntity":{"__typename":"Show","videoId":82068293}}},
					{"node":{"__typename":"PinotRankedBoxshotEntityTreatment","displayString":"Te conozco",
						"unifiedEntity":{"__typename":"Show","videoId":81726799}}}]}}}]}}}}`,
	})
	code, out := runCommand(t, "top")
	if code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out, " 1. Monstruo") || !strings.Contains(out, " 2. Te conozco") {
		t.Errorf("output %q does not number the ranking", out)
	}
	if strings.Contains(out, "Dark") {
		t.Errorf("output %q includes an unranked row", out)
	}
}

func TestTopReportsWhenNetflixRendersNoRanking(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"FetchMoreSections": `{"data":{"page":{"sections":{"pageInfo":{"hasNextPage":false},"edges":[]}}}}`,
	})
	code, _ := runCommand(t, "top")
	if code != exitError {
		t.Errorf("exit code = %d, want %d when no ranked row came back", code, exitError)
	}
}

func TestBrowseAllPagesPastThePage(t *testing.T) {
	stubNetflix(t)
	stubGateway(t, map[string]string{
		"FetchMoreSections": `{"data":{"page":{"sections":{
			"pageInfo":{"endCursor":"","hasNextPage":false},
			"edges":[{"node":{"displayString":"Fila profunda","entities":{"edges":[
				{"node":{"displayString":"Fariña","unifiedEntity":{"__typename":"Show","videoId":80215500}}}]}}}]}}}}`,
	})
	code, out := runCommand(t, "browse", "--all")
	if code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out, "Fila profunda") {
		t.Errorf("output %q does not carry the fetched row", out)
	}
}
