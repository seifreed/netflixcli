package client

import "testing"

func TestParseThumbRating(t *testing.T) {
	cases := []struct{ word, want string }{
		{"up", "THUMBS_UP"},
		{"UP", "THUMBS_UP"},
		{"  down  ", "THUMBS_DOWN"}, // surrounding space is trimmed
		{"love", "THUMBS_WAY_UP"},
		{"way-up", "THUMBS_WAY_UP"},
		{"none", "THUMBS_UNRATED"},
		{"unrated", "THUMBS_UNRATED"},
	}
	for _, tc := range cases {
		word, want := tc.word, tc.want
		got, err := parseThumbRating(word)
		if err != nil {
			t.Errorf("parseThumbRating(%q): %v", word, err)
			continue
		}
		if got != want {
			t.Errorf("parseThumbRating(%q) = %q, want %q", word, got, want)
		}
	}
	if _, err := parseThumbRating("sideways"); err == nil {
		t.Error("want an error for an unknown rating")
	}
}

func TestEntityEnvelopeSurfacesNetflixErrors(t *testing.T) {
	var env entityEnvelope
	env.Errors = append(env.Errors, struct {
		Message string `json:"message"`
	}{Message: "not allowed"})
	if _, err := env.state(); err == nil {
		t.Fatal("want an error when Netflix reports one in the payload")
	}
}

func TestEntityEnvelopeState(t *testing.T) {
	var env entityEnvelope
	env.Entity.VideoID = 70095139
	env.Entity.Title = "Shutter Island"
	env.Entity.IsInPlaylist = true
	got, err := env.state()
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	want := EntityState{ID: 70095139, Title: "Shutter Island", InMyList: true, URL: "https://www.netflix.com/title/70095139"}
	if got != want {
		t.Errorf("state = %+v, want %+v", got, want)
	}
}
