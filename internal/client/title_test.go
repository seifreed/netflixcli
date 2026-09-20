package client

import "testing"

func TestParseTitleID(t *testing.T) {
	for raw, want := range map[string]int{
		"70095139":                                    70095139,
		"https://www.netflix.com/title/70095139":      70095139,
		"https://www.netflix.com/es/title/70095139/":  70095139,
		"https://www.netflix.com/watch/80100172?t=10": 80100172,
		"/title/80100172":                             80100172,
	} {
		got, err := ParseTitleID(raw)
		if err != nil {
			t.Errorf("ParseTitleID(%q): %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("ParseTitleID(%q) = %d, want %d", raw, got, want)
		}
	}
	for _, raw := range []string{"", "   ", "not-an-id", "https://www.netflix.com/browse", "-5"} {
		if id, err := ParseTitleID(raw); err == nil {
			t.Errorf("ParseTitleID(%q) = %d, want an error", raw, id)
		}
	}
}

func TestTitleDetailRuntime(t *testing.T) {
	for secs, want := range map[int]string{
		0:    "",
		-1:   "",
		2880: "48m",
		7942: "2h 12m",
		3600: "1h 00m",
	} {
		if got := (TitleDetail{RuntimeSec: secs}).Runtime(); got != want {
			t.Errorf("Runtime(%d) = %q, want %q", secs, got, want)
		}
	}
}

func TestEntityID(t *testing.T) {
	if got := entityID(70095139); got != "Video:70095139" {
		t.Errorf("entityID = %q, want Video:70095139", got)
	}
}
