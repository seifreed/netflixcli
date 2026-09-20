package main

import (
	"testing"

	"github.com/seifreed/netflixcli/internal/client"
)

// --csv is meant to be piped into a spreadsheet, so a title carrying a comma or
// a quote must not shift the columns of the row it is on.
func TestViewingsCSVQuotesWhatWouldBreakTheRow(t *testing.T) {
	out := captureStdout(t, func() {
		err := emitViewingsCSV([]client.Viewing{
			{Date: "2026-01-02", Title: "Dark"},
			{Date: "2026-01-03", Title: `Ozark: Season 1, Episode 2`},
			{Date: "2026-01-04", Title: `The "Good" Place`},
		})
		if err != nil {
			t.Errorf("emitViewingsCSV: %v", err)
		}
	})
	want := "date,title\n" +
		"2026-01-02,Dark\n" +
		"2026-01-03,\"Ozark: Season 1, Episode 2\"\n" +
		"2026-01-04,\"The \"\"Good\"\" Place\"\n"
	if out != want {
		t.Errorf("csv =\n%q\nwant\n%q", out, want)
	}
}

func TestViewingsCSVOfAnEmptyHistoryIsJustTheHeader(t *testing.T) {
	out := captureStdout(t, func() {
		if err := emitViewingsCSV(nil); err != nil {
			t.Errorf("emitViewingsCSV: %v", err)
		}
	})
	if out != "date,title\n" {
		t.Errorf("csv = %q, want the header alone", out)
	}
}
