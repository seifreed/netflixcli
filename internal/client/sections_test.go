package client

import (
	"encoding/base64"
	"testing"
)

// A top-10 row is identified by its card treatment, because the row title is
// localised ("Las 10 series más populares hoy en este país: España").
func TestRankedRowsAreFoundByTreatment(t *testing.T) {
	const page = `{"page":{"sections":{
		"pageInfo":{"endCursor":"next","hasNextPage":true},
		"edges":[
			{"node":{"__typename":"PinotCarouselSection","displayString":"Series dramáticas","entities":{"edges":[
				{"node":{"__typename":"PinotStandardBoxshotEntityTreatment","displayString":"Dark",
					"unifiedEntity":{"__typename":"Show","videoId":80100172}}}]}}},
			{"node":{"__typename":"PinotCarouselSection","displayString":"Las 10 series más populares","entities":{"edges":[
				{"node":{"__typename":"PinotRankedBoxshotEntityTreatment","displayString":"Monstruo",
					"unifiedEntity":{"__typename":"Show","videoId":82068293}}}]}}}]}}}`
	var parsed pinotPage
	mustUnmarshal(t, page, &parsed)

	rows := parsed.rows()
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].Ranked {
		t.Error("an ordinary row was reported as a ranking")
	}
	if !rows[1].Ranked {
		t.Error("the top-10 row was not recognised as a ranking")
	}
	if parsed.Page.Sections.PageInfo.EndCursor != "next" {
		t.Error("the sections cursor was not decoded")
	}
}

// The ranked flag has to survive the other path too: rows read from the page's
// own cache, not fetched over GraphQL.
func TestRankedRowsAreFoundInThePageCache(t *testing.T) {
	const page = `<script>netflix.reactContext.models.graphql = JSON.parse('{"data":{` +
		`"ROOT_QUERY":{"pinotBrowsePage({})":{"__ref":"Page:1"}},` +
		`"Page:1":{"sections":{"edges":[{"node":{"__ref":"S:1"}}]}},` +
		`"S:1":{"displayString":"Top 10","entities":{"edges":[{"node":{"__ref":"C:1"}}]}},` +
		`"C:1":{"__typename":"PinotRankedBoxshotEntityTreatment","displayString":"Monstruo",` +
		`"unifiedEntity":{"__ref":"V:1"}},` +
		`"V:1":{"__typename":"Show","videoId":82068293}` +
		`}}');</script>`
	cache, err := parseApolloCache(page)
	if err != nil {
		t.Fatalf("parseApolloCache: %v", err)
	}
	rows := cache.rows()
	if len(rows) != 1 || !rows[0].Ranked {
		t.Fatalf("rows = %+v, want one ranked row", rows)
	}
}

func TestSectionVarsAreValid(t *testing.T) {
	vars := sectionVars()
	// FetchMoreSections declares every artwork variable non-null; a missing one
	// makes the page assembler fail rather than answer.
	if len(vars) != 23 {
		t.Errorf("got %d artwork variables, want the 23 the query declares", len(vars))
	}
}

func TestOptionalString(t *testing.T) {
	if optionalString("") != nil {
		t.Error("an absent cursor must be sent as null")
	}
	if optionalString("abc") != "abc" {
		t.Error("a cursor must be sent as itself")
	}
}

// `browse --all` answers for the same surface `browse` does, so it must not
// answer with less. It used to drop every row's feed and every empty personal
// row, so My Netflix came back one row short with nothing naming the lists.
func TestFetchedRowsCarryTheFeedAndKeepEmptyPersonalRows(t *testing.T) {
	var page pinotPage
	mustUnmarshal(t, `{"page":{"sections":{"edges":[
	 {"node":{"__typename":"PinotCarouselSection","_id":"s1","displayString":"Mi lista",
	   "eventListeners":[{"actions":[{"id":"`+base64Of("k playlist v")+`"}]}],
	   "entities":{"edges":[{"node":{"displayString":"Dark","unifiedEntity":{"__typename":"Show","videoId":80100172}}}]}}},
	 {"node":{"__typename":"PinotCarouselSection","_id":"s2","displayString":"Recordatorios programados",
	   "eventListeners":[{"actions":[{"id":"`+base64Of("x reminders y")+`"}]}],
	   "entities":{"edges":[]}}},
	 {"node":{"__typename":"PinotCarouselSection","_id":"s3","displayString":"Una fila editorial vacía",
	   "entities":{"edges":[]}}}]}}}`, &page)

	rows := page.rows()
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want the empty personal row kept and the empty editorial one dropped: %+v", len(rows), rows)
	}
	if rows[0].Feed != FeedMyList {
		t.Errorf("row 0 feed = %q, want %q", rows[0].Feed, FeedMyList)
	}
	if rows[1].Feed != FeedReminders || len(rows[1].Titles) != 0 {
		t.Errorf("row 1 = %+v, want the empty reminders row", rows[1])
	}
}

func base64Of(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
