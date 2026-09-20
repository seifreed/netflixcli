package main

import (
	"testing"

	"github.com/seifreed/netflixcli/internal/client"
)

// --limit shows a slice of each row. The rows it was given must survive intact,
// or a later reader of the same slice would silently see the capped view.
func TestCapRowsLeavesTheRowsItWasGivenAlone(t *testing.T) {
	rows := []client.Row{{
		Name:   "Trending",
		Titles: []client.Title{{ID: 1}, {ID: 2}, {ID: 3}},
	}}
	capped := capRows(rows, 2)
	if len(capped[0].Titles) != 2 {
		t.Errorf("capped row has %d titles, want 2", len(capped[0].Titles))
	}
	if len(rows[0].Titles) != 3 {
		t.Errorf("the original row was cut to %d titles", len(rows[0].Titles))
	}
}

func TestCapRowsWithoutALimitReturnsEverything(t *testing.T) {
	rows := []client.Row{{Titles: []client.Title{{ID: 1}, {ID: 2}}}}
	if got := capRows(rows, 0); len(got[0].Titles) != 2 {
		t.Errorf("limit 0 cut the row to %d titles", len(got[0].Titles))
	}
}
