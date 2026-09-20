package client

import "testing"

func TestParseHistoryCSV(t *testing.T) {
	raw := "Title,Date\n" +
		"\"Fariña: Capítulo 5: 1985\",\"9/16/26\"\n" +
		"\"A title, with a comma\",\"12/1/25\"\n" +
		"\"Odd date\",\"sometime\"\n"
	got, err := parseHistoryCSV(raw)
	if err != nil {
		t.Fatalf("parseHistoryCSV: %v", err)
	}
	want := []Viewing{
		{Title: "Fariña: Capítulo 5: 1985", Date: "2026-09-16"},
		{Title: "A title, with a comma", Date: "2025-12-01"},
		{Title: "Odd date", Date: "sometime"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseHistoryCSVEmpty(t *testing.T) {
	got, err := parseHistoryCSV("Title,Date\n")
	if err != nil {
		t.Fatalf("parseHistoryCSV: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries from a header-only CSV, want 0", len(got))
	}
}
