package client

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestParseTitleID(t *testing.T) {
	for raw, want := range map[string]int{
		"70095139":                                    70095139,
		"https://www.netflix.com/title/70095139":      70095139,
		"https://www.netflix.com/es/title/70095139/":  70095139,
		"https://www.netflix.com/watch/80100172?t=10": 80100172,
		"/title/80100172":                             80100172,
	} {
		got, err := ParseTitleID(raw)
		if err != nil {
			t.Errorf("ParseTitleID(%q): %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("ParseTitleID(%q) = %d, want %d", raw, got, want)
		}
	}
	for _, raw := range []string{"", "   ", "not-an-id", "https://www.netflix.com/browse", "-5"} {
		if id, err := ParseTitleID(raw); err == nil {
			t.Errorf("ParseTitleID(%q) = %d, want an error", raw, id)
		}
	}
}

func TestTitleDetailRuntime(t *testing.T) {
	for secs, want := range map[int]string{
		0:    "",
		-1:   "",
		2880: "48m",
		7942: "2h 12m",
		3600: "1h 00m",
	} {
		if got := (TitleDetail{RuntimeSec: secs}).Runtime(); got != want {
			t.Errorf("Runtime(%d) = %q, want %q", secs, got, want)
		}
	}
}

func TestEntityID(t *testing.T) {
	if got := entityID(70095139); got != "Video:70095139" {
		t.Errorf("entityID = %q, want Video:70095139", got)
	}
}

// Details fills a row of cards in one request. Netflix answers with whatever it
// has, so an entity without a video id — a header, a game with no video — must
// be dropped rather than reported as a title with id 0.
func TestDetailsSkipsEntitiesWithoutAVideoID(t *testing.T) {
	var sent map[string]any
	c := graphQLClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(raw, &body)
		sent = body.Variables
		if _, err := w.Write([]byte(`{"data":{"unifiedEntities":[
			{"__typename":"Show","videoId":80100172,"title":"Dark"},
			{"__typename":"GenericContainer","videoId":0,"title":"Colecciones"},
			{"__typename":"Movie","videoId":70095139,"title":"Fariña"}]}}`)); err != nil {
			t.Errorf("stub write: %v", err)
		}
	})
	c.queries.Ops["MiniModalQuery"] = "mini-1"

	details, err := c.Catalog.Details([]int{80100172, 0, 70095139})
	if err != nil {
		t.Fatalf("Details: %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("got %d details, want the id-less entity dropped", len(details))
	}
	if details[0].Title != "Dark" || details[1].URL != TitleURL(70095139) {
		t.Errorf("details = %+v, want Dark and Fariña with its url filled in", details)
	}
	ids, _ := sent["unifiedEntityIds"].([]any)
	if len(ids) != 3 || ids[0] != "Video:80100172" {
		t.Errorf("sent ids %v, want every requested id as a unified entity id", ids)
	}
}

// No ids means no request: the gateway would answer an empty row anyway.
func TestDetailsOfNothingAsksNothing(t *testing.T) {
	c := graphQLClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("Details sent a request for an empty id list")
	})
	details, err := c.Catalog.Details(nil)
	if err != nil || details != nil {
		t.Errorf("Details(nil) = %v, %v, want nothing", details, err)
	}
}
