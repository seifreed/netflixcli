package client

import "testing"

func TestMatchGenres(t *testing.T) {
	genres := []Genre{
		{Title: "Películas de terror", Number: 8711},
		{Title: "Series de terror", Number: 83059},
		{Title: "Comedias", Number: 6548},
	}
	if got := MatchGenres(genres, ""); len(got) != 3 {
		t.Errorf("an empty query kept %d genres, want all 3", len(got))
	}
	got := MatchGenres(genres, "TERROR")
	if len(got) != 2 {
		t.Fatalf("matched %d genres, want 2 (the match is case-insensitive)", len(got))
	}
	if MatchGenres(genres, "documentales") != nil {
		t.Error("want nil when nothing matches")
	}
}
