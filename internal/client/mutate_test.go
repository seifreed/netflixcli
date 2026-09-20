package client

import "testing"

func TestParseThumbRating(t *testing.T) {
	for word, want := range map[string]string{
		"up":      "THUMBS_UP",
		"UP":      "THUMBS_UP",
		" down ":  "THUMBS_DOWN",
		"love":    "THUMBS_WAY_UP",
		"way-up":  "THUMBS_WAY_UP",
		"none":    "THUMBS_UNRATED",
		"unrated": "THUMBS_UNRATED",
	} {
		got, err := ParseThumbRating(word)
		if err != nil {
			t.Errorf("ParseThumbRating(%q): %v", word, err)
			continue
		}
		if got != want {
			t.Errorf("ParseThumbRating(%q) = %q, want %q", word, got, want)
		}
	}
	if _, err := ParseThumbRating("sideways"); err == nil {
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
