package client

import (
	"fmt"
	"strconv"
	"strings"
)

// Browse surfaces, named as the CLI exposes them. Each maps to the page the web
// app renders for that tab.
const (
	SurfaceHome      = "home"
	SurfaceMyNetflix = "my-netflix"
	SurfaceLatest    = "latest"
	SurfaceGames     = "games"
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
// is, when it is one (see the Feed constants); editorial rows have none.
type Row struct {
	Name   string  `json:"name"`
	Feed   string  `json:"feed,omitempty"`
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
	case SurfaceLatest, "new":
		return "/latest", nil
	case SurfaceGames:
		return "/browse/games", nil
	}
	if id, err := strconv.Atoi(strings.TrimPrefix(surface, "genre-")); err == nil && id > 0 {
		return fmt.Sprintf("/browse/genre/%d", id), nil
	}
	return "", fmt.Errorf("unknown browse surface %q (want home, my-netflix, latest, games or a genre id)", surface)
}

// Browse returns the rows of a browse surface, read from the page Netflix
// renders for it.
func (c *Client) Browse(surface string) ([]Row, error) {
	path, err := surfacePath(surface)
	if err != nil {
		return nil, err
	}
	if c.Cookie == "" {
		return nil, ErrNoSession
	}
	html, err := c.GetText(c.BaseURL + path)
	if err != nil {
		return nil, err
	}
	cache, err := parseApolloCache(html)
	if err != nil {
		return nil, err
	}
	return cache.rows(), nil
}

// Feed returns one personal row of the My Netflix page — My List, Continue
// Watching, liked titles, reminders or watched trailers.
//
// Netflix occasionally renders the page without one of these rows, so a miss is
// retried once before it is reported: a single reload has been enough every
// time it has been observed.
func (c *Client) Feed(feed string) (Row, error) {
	for attempt := 0; attempt < 2; attempt++ {
		rows, err := c.Browse(SurfaceMyNetflix)
		if err != nil {
			return Row{}, err
		}
		for _, row := range rows {
			if row.Feed == feed {
				return row, nil
			}
		}
		c.logf("netflix rendered My Netflix without the %q row — reloading", feed)
	}
	return Row{}, fmt.Errorf("netflix did not render a %q row for this profile", feed)
}
