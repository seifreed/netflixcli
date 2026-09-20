package client

import (
	"fmt"
	"net/http"
	"testing"
)

// A search page is one gallery of results plus a suggestions list; only the
// gallery carries video ids, and it is what pagination continues from.
const searchPageJSON = `{"page":{"sections":{"edges":[
 {"node":{"__typename":"PinotListSection","displayString":"Suggestions","entities":{"edges":[
   {"node":{"__typename":"PinotSuggestionEntityTreatment","displayString":"inception"}}]}}},
 {"node":{"__typename":"PinotGallerySection","_id":"gallery-1","displayString":"Films",
   "entities":{"totalCount":299,"pageInfo":{"endCursor":"NDc=","hasNextPage":true},"edges":[
     {"node":{"displayString":"Shutter Island","unifiedEntity":{"__typename":"Movie","videoId":70095139}}},
     {"node":{"displayString":"A header","unifiedEntity":{"__typename":"GenericContainer"}}}]}}}]}}}`

func TestGallerySectionIsolatesTheResultGallery(t *testing.T) {
	var page pinotPage
	mustUnmarshal(t, searchPageJSON, &page)
	gallery, ok := page.gallerySection()
	if !ok {
		t.Fatal("want the gallery section to be found")
	}
	if gallery.ID != "gallery-1" {
		t.Errorf("gallery id = %q, want the node id pagination needs", gallery.ID)
	}
	if !gallery.Entities.PageInfo.HasNextPage || gallery.Entities.PageInfo.EndCursor != "NDc=" {
		t.Errorf("pageInfo = %+v, want the cursor kept", gallery.Entities.PageInfo)
	}
	titles := gallery.titles()
	if len(titles) != 1 || titles[0].ID != 70095139 {
		t.Errorf("titles = %+v, want only the card carrying a video id", titles)
	}
}

func TestGallerySectionMissing(t *testing.T) {
	var page pinotPage
	mustUnmarshal(t, `{"page":{"sections":{"edges":[]}}}`, &page)
	if _, ok := page.gallerySection(); ok {
		t.Error("want ok=false when the page has no gallery")
	}
}

// searchStub answers the first request with a gallery and every later one with
// the next slice, counting how many times it was asked.
func searchStub(t *testing.T, pages []string) (*Client, *int) {
	t.Helper()
	calls := 0
	c := graphQLClient(t, func(w http.ResponseWriter, _ *http.Request) {
		page := pages[len(pages)-1]
		if calls < len(pages) {
			page = pages[calls]
		}
		calls++
		if _, err := w.Write([]byte(page)); err != nil {
			t.Errorf("stub write: %v", err)
		}
	})
	c.queries.Ops["SearchPageQueryResults"] = "search-1"
	c.queries.Ops["FetchMoreSearchGalleryItems"] = "more-1"
	return c, &calls
}

// gallerySlice renders one FetchMoreSearchGalleryItems answer.
func gallerySlice(cursor string, more bool, ids ...int) string {
	cards := ""
	for i, id := range ids {
		if i > 0 {
			cards += ","
		}
		cards += fmt.Sprintf(`{"node":{"displayString":"T%d","unifiedEntity":{"__typename":"Movie","videoId":%d}}}`, id, id)
	}
	return fmt.Sprintf(`{"data":{"node":{"__typename":"PinotGallerySection","_id":"gallery-1",
		"entities":{"pageInfo":{"endCursor":%q,"hasNextPage":%t},"edges":[%s]}}}}`, cursor, more, cards)
}

// A limit past the first page keeps asking until it has enough.
func TestSearchPagesUntilItHasTheLimit(t *testing.T) {
	c, calls := searchStub(t, []string{
		`{"data":` + searchPageJSON + `}`,
		gallerySlice("c2", true, 1, 2),
		gallerySlice("c3", false, 3, 4),
	})

	titles, err := c.Catalog.Search("dark", 4)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if *calls != 3 {
		t.Errorf("made %d requests, want 3", *calls)
	}
	if len(titles) != 4 {
		t.Fatalf("got %d titles, want the limit honoured exactly", len(titles))
	}
	if titles[0].ID != 70095139 || titles[3].ID != 3 {
		t.Errorf("titles = %+v, want the first page kept ahead of the fetched ones", titles)
	}
}

// Without a limit the first page is the answer; nothing is fetched after it.
func TestSearchWithoutALimitAsksOnce(t *testing.T) {
	c, calls := searchStub(t, []string{`{"data":` + searchPageJSON + `}`})

	titles, err := c.Catalog.Search("dark", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if *calls != 1 || len(titles) != 1 {
		t.Errorf("%d requests for %d titles, want one page", *calls, len(titles))
	}
}

// A cursor that stops advancing hands back the titles already seen. Without the
// guard on new results, the loop would run until the limit it can never reach.
func TestSearchStopsWhenAPageAddsNothingNew(t *testing.T) {
	c, calls := searchStub(t, []string{
		`{"data":` + searchPageJSON + `}`,
		gallerySlice("stuck", true, 70095139),
	})

	titles, err := c.Catalog.Search("dark", 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if *calls != 2 {
		t.Errorf("made %d requests, want it to stop after the repeat", *calls)
	}
	if len(titles) != 1 {
		t.Errorf("got %d titles, want the duplicate dropped", len(titles))
	}
}

func TestSearchNeedsAQuery(t *testing.T) {
	c, calls := searchStub(t, []string{`{"data":` + searchPageJSON + `}`})
	if _, err := c.Catalog.Search("   ", 0); err == nil {
		t.Error("want an error for a blank query")
	}
	if *calls != 0 {
		t.Error("a blank query reached the gateway")
	}
}
