package client

import "testing"

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
