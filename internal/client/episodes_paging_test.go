package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// episodeStub serves a season of the given length, honouring the count and
// cursor each request asks with, and records what was asked.
func episodeStub(t *testing.T, seasonLen int) (*Client, *[]int) {
	t.Helper()
	var counts []int
	c := graphQLClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Variables struct {
				Count  int    `json:"count"`
				Cursor string `json:"cursor"`
			} `json:"variables"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		counts = append(counts, body.Variables.Count)
		from := 0
		if body.Variables.Cursor != "" {
			if _, err := fmt.Sscanf(body.Variables.Cursor, "at-%d", &from); err != nil {
				t.Errorf("unexpected cursor %q", body.Variables.Cursor)
			}
		}
		to := from + body.Variables.Count
		if to > seasonLen {
			to = seasonLen
		}
		edges := make([]string, 0, to-from)
		for i := from; i < to; i++ {
			edges = append(edges, fmt.Sprintf(`{"node":{"videoId":%d,"number":%d,"title":"E%d"}}`, 900+i, i+1, i+1))
		}
		fmt.Fprintf(w, `{"data":{"videos":[{"episodes":{"edges":[%s],
			"pageInfo":{"endCursor":"at-%d","hasNextPage":%t}}}]}}`,
			strings.Join(edges, ","), to, to < seasonLen)
	})
	c.queries.Ops["PreviewModalEpisodeSelectorSeasonEpisodes"] = "eps-1"
	return c, &counts
}

// A season longer than one request's worth is paged, not cut off. Netflix says
// this one has 63 episodes; it used to answer with 50 and say nothing.
func TestEpisodesPagesAWholeLongSeason(t *testing.T) {
	c, counts := episodeStub(t, 63)

	episodes, err := c.Catalog.Episodes(70190838, 0)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(episodes) != 63 {
		t.Fatalf("got %d episodes, want the whole season of 63", len(episodes))
	}
	if episodes[0].Number != 1 || episodes[62].Number != 63 {
		t.Errorf("numbered %d..%d, want 1..63", episodes[0].Number, episodes[62].Number)
	}
	if want := []int{50, 50}; fmt.Sprint(*counts) != fmt.Sprint(want) {
		t.Errorf("asked for %v per request, want %v", *counts, want)
	}
}

// An explicit limit is honoured exactly, above and below the page size. It used
// to be clamped to the page size, so --limit 100 returned 50.
func TestEpisodesHonoursTheLimitEitherSideOfThePageSize(t *testing.T) {
	for _, tc := range []struct {
		limit      int
		want       int
		wantCounts []int
	}{
		{limit: 3, want: 3, wantCounts: []int{3}},
		{limit: 50, want: 50, wantCounts: []int{50}},
		{limit: 60, want: 60, wantCounts: []int{50, 10}},
		{limit: 100, want: 63, wantCounts: []int{50, 50}}, // the second page answers 13 and the season ends
	} {
		c, counts := episodeStub(t, 63)
		episodes, err := c.Catalog.Episodes(70190838, tc.limit)
		if err != nil {
			t.Fatalf("limit %d: %v", tc.limit, err)
		}
		if len(episodes) != tc.want {
			t.Errorf("limit %d returned %d episodes, want %d", tc.limit, len(episodes), tc.want)
		}
		if fmt.Sprint(*counts) != fmt.Sprint(tc.wantCounts) {
			t.Errorf("limit %d asked %v, want %v", tc.limit, *counts, tc.wantCounts)
		}
	}
}

// A cursor that stops advancing must end the loop rather than run to the page
// bound asking the same thing.
func TestEpisodesStopsOnAStuckCursor(t *testing.T) {
	calls := 0
	c := graphQLClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		fmt.Fprint(w, `{"data":{"videos":[{"episodes":{
			"edges":[{"node":{"videoId":901,"number":1,"title":"E1"}}],
			"pageInfo":{"endCursor":"stuck","hasNextPage":true}}}]}}`)
	})
	c.queries.Ops["PreviewModalEpisodeSelectorSeasonEpisodes"] = "eps-1"

	episodes, err := c.Catalog.Episodes(70190838, 0)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if calls != 2 {
		t.Errorf("made %d requests, want it to stop once the cursor repeated", calls)
	}
	if len(episodes) != 2 {
		t.Errorf("got %d episodes from two pages", len(episodes))
	}
}
