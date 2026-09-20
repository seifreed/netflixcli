package client

import (
	"fmt"
	"strconv"
	"strings"
)

// Browse surfaces, named as the CLI exposes them. Each maps to a page whose
// rows Netflix server-renders; /latest and /browse/games do not (they ship an
// empty cache and fetch their rows client-side from a query that answers a
// server-side error for static categories), so they are not offered.
const (
	SurfaceHome      = "home"
	SurfaceMyNetflix = "my-netflix"
)

// Feed ids Netflix gives the personal rows of the My Netflix page. The row
// titles are localised, so these are what the CLI matches on.
const (
	FeedMyList           = "playlist"
	FeedContinueWatching = "continuewatching"
	FeedLiked            = "favoritetitles"
	FeedReminders        = "reminders"
	FeedTrailers         = "trailers"
)

// Row is one titled carousel of a browse page. Feed names the personal list it
// is, when it is one (see the Feed constants); editorial rows have none. Ranked
// marks a row whose order is a ranking — Netflix's top 10 lists.
type Row struct {
	Name   string  `json:"name"`
	Feed   string  `json:"feed,omitempty"`
	Ranked bool    `json:"ranked,omitempty"`
	Titles []Title `json:"titles"`
}

// surfacePath maps a surface name or genre id to the page that carries it.
func surfacePath(surface string) (string, error) {
	surface = strings.TrimSpace(strings.ToLower(surface))
	switch surface {
	case "", SurfaceHome:
		return "/browse", nil
	case SurfaceMyNetflix, "my-list", "mylist":
		return "/browse/my-list", nil
	}
	if id, err := strconv.Atoi(strings.TrimPrefix(surface, "genre-")); err == nil && id > 0 {
		return fmt.Sprintf("/browse/genre/%d", id), nil
	}
	return "", fmt.Errorf("unknown browse surface %q (want home, my-netflix or a genre id — run `netflix genres` for the ids)", surface)
}

// Browse returns the rows a browse surface renders — the eight Netflix puts in
// the page. AllRows fetches the rest.
func (s *Library) Browse(surface string) ([]Row, error) {
	cache, err := s.surfaceCache(surface)
	if err != nil {
		return nil, err
	}
	return cache.rows(), nil
}

// surfaceCache loads a surface's page and returns the row cache embedded in it.
func (s *Library) surfaceCache(surface string) (apolloCache, error) {
	path, err := surfacePath(surface)
	if err != nil {
		return nil, err
	}
	if s.client.Cookie == "" {
		return nil, ErrNoSession
	}
	html, err := s.client.getText(s.client.BaseURL + path)
	if err != nil {
		return nil, err
	}
	return parseApolloCache(html)
}

// Feed returns one personal row of the My Netflix page — My List, Continue
// Watching, liked titles, reminders or watched trailers.
//
// Netflix occasionally renders the page without one of these rows, so a miss is
// retried once before it is reported: a single reload has been enough every
// time it has been observed.
func (s *Library) Feed(feed string) (Row, error) {
	for attempt := 0; attempt < 2; attempt++ {
		rows, err := s.Browse(SurfaceMyNetflix)
		if err != nil {
			return Row{}, err
		}
		for _, row := range rows {
			if row.Feed == feed {
				return row, nil
			}
		}
		s.client.logf("netflix rendered My Netflix without the %q row — reloading", feed)
	}
	return Row{}, fmt.Errorf("netflix did not render a %q row for this profile", feed)
}
