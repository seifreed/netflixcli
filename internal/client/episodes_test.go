package client

import "testing"

func TestSeasonByNumber(t *testing.T) {
	seasons := []Season{{Number: 1, ID: 11}, {Number: 2, ID: 22}, {Number: 3, ID: 33}}
	got, err := SeasonByNumber(seasons, 2)
	if err != nil {
		t.Fatalf("SeasonByNumber: %v", err)
	}
	if got.ID != 22 {
		t.Errorf("season id = %d, want 22", got.ID)
	}
	if _, err := SeasonByNumber(seasons, 9); err == nil {
		t.Error("want an error for a season the show does not have")
	}
}

func TestEpisodeRuntime(t *testing.T) {
	if got := (Episode{RuntimeSec: 3091}).Runtime(); got != "51m" {
		t.Errorf("Runtime = %q, want 51m", got)
	}
	if got := (Episode{}).Runtime(); got != "" {
		t.Errorf("Runtime = %q, want empty when unknown", got)
	}
}
