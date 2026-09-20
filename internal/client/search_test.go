package client

import "testing"

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
