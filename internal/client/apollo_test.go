package client

import "testing"

// A Netflix page in miniature: the Apollo cache as a single-quoted JavaScript
// literal (so \" arrives escaped), fields keyed with their arguments, entities
// reached through __ref, and the feed identity hidden in a base64 action id.
const pageWithRows = `<script>netflix.reactContext.models.graphql = JSON.parse('{"data":{` +
	`"ROOT_QUERY":{"pinotBrowsePage({\\"categoryId\\":\\"myProfile\\"})":{"__ref":"Page:1"}},` +
	`"Page:1":{"__typename":"PinotDefaultBrowsePage","sections({\\"first\\":8})":{"edges":[{"node":{"__ref":"Section:1"}},{"node":{"__ref":"Section:2"}}]}},` +
	`"Section:1":{"__typename":"PinotCarouselSection","displayString":"Mi lista",` +
	`"eventListeners":[{"__typename":"PinotAddToPlaylistEventListener","actions":[{"__ref":"PinotPageUpdateAction:CghwbGF5bGlzdBICCDc="}]}],` +
	`"entities":{"edges":[{"node":{"__ref":"Card:1"}},{"node":{"__ref":"Card:2"}}]}},` +
	`"Section:2":{"__typename":"PinotCarouselSection","displayString":"Editorial","entities":{"edges":[{"node":{"__ref":"Card:3"}}]}},` +
	`"Card:1":{"displayString":"Dark","unifiedEntity":{"__ref":"Show:1"},"contextualArtwork":{"artwork":{"url":"https://img.example/dark.jpg"}}},` +
	`"Card:2":{"displayString":"A header with no video","unifiedEntity":{"__ref":"Header:1"}},` +
	`"Card:3":{"displayString":"Fariña","unifiedEntity":{"__ref":"Show:2"}},` +
	`"Show:1":{"__typename":"Show","videoId":80100172,"contentAdvisory":{"maturityLevel":90}},` +
	`"Header:1":{"__typename":"GenericContainer"},` +
	`"Show:2":{"__typename":"Show","videoId":80215500}` +
	`}}');</script>`

func TestApolloCacheRows(t *testing.T) {
	cache, err := parseApolloCache(pageWithRows)
	if err != nil {
		t.Fatalf("parseApolloCache: %v", err)
	}
	rows := cache.rows()
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}

	myList := rows[0]
	if myList.Name != "Mi lista" {
		t.Errorf("row name = %q, want the localised title", myList.Name)
	}
	if myList.Feed != FeedMyList {
		t.Errorf("row feed = %q, want %q decoded from the action id", myList.Feed, FeedMyList)
	}
	if len(myList.Titles) != 1 {
		t.Fatalf("got %d titles, want the card without a video id dropped", len(myList.Titles))
	}
	got := myList.Titles[0]
	want := Title{
		ID: 80100172, Title: "Dark", Kind: "Show",
		URL: "https://www.netflix.com/title/80100172", MaturityLevel: 90,
		Artwork: "https://img.example/dark.jpg",
	}
	if got != want {
		t.Errorf("title = %+v, want %+v", got, want)
	}
	if rows[1].Feed != "" {
		t.Errorf("editorial row feed = %q, want empty", rows[1].Feed)
	}
}

func TestParseApolloCacheRejectsPageWithoutData(t *testing.T) {
	if _, err := parseApolloCache("<html><body>sign in</body></html>"); err == nil {
		t.Fatal("want an error when the page carries no Apollo cache")
	}
}

func TestUnescapeJSString(t *testing.T) {
	for in, want := range map[string]string{
		`plain`:           `plain`,
		`{\"a\":1}`:       `{"a":1}`,
		`back\\slash`:     `back\slash`,
		`it\'s`:           `it's`,
		`\\\"escaped\\\"`: `\"escaped\"`,
	} {
		if got := unescapeJSString(in); got != want {
			t.Errorf("unescapeJSString(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestSurfacePath(t *testing.T) {
	for surface, want := range map[string]string{
		"":           "/browse",
		"home":       "/browse",
		"my-netflix": "/browse/my-list",
		"mylist":     "/browse/my-list",
		"83":         "/browse/genre/83",
		"genre-83":   "/browse/genre/83",
	} {
		got, err := surfacePath(surface)
		if err != nil {
			t.Errorf("surfacePath(%q): %v", surface, err)
			continue
		}
		if got != want {
			t.Errorf("surfacePath(%q) = %q, want %q", surface, got, want)
		}
	}
	// /latest and /browse/games carry no rows, so they are not surfaces.
	for _, surface := range []string{"nonsense", "latest", "games"} {
		if _, err := surfacePath(surface); err == nil {
			t.Errorf("surfacePath(%q) was accepted, want a refusal", surface)
		}
	}
}

// A page can key the same field under more than one set of arguments. Ranging
// over the map picked between them by iteration order, so the same page could
// answer differently from one run to the next.
func TestFieldPicksTheSameVariantEveryTime(t *testing.T) {
	obj := map[string]any{
		`sections({"first":8})`:       "eight",
		`sections({"first":20})`:      "twenty",
		`sections({"first":40})`:      "forty",
		`sectionsOther({"first":99})`: "unrelated",
	}
	first, _ := field(obj, "sections").(string)
	if first == "" {
		t.Fatal("field found no variant at all")
	}
	for i := 0; i < 1000; i++ {
		if got, _ := field(obj, "sections").(string); got != first {
			t.Fatalf("field returned %q then %q for the same object", first, got)
		}
	}
	// An exact key still wins over any argument-keyed variant.
	obj["sections"] = "exact"
	if got, _ := field(obj, "sections").(string); got != "exact" {
		t.Errorf("field = %q, want the exact key to win", got)
	}
}
