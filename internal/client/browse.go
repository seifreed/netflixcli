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

// defaultFeedPageSize is how many titles a feed asks for before it knows how
// long the list is. It covers most lists in one request.
const defaultFeedPageSize = 100

// Feed returns one personal row of the My Netflix page — My List, Continue
// Watching, liked titles, reminders or watched trailers — with the whole list
// in it. limit stops it short; 0 asks for all of it.
//
// The carousel the page renders carries thirteen titles whatever the list
// holds, so reading the row out of the page answered `mylist` with thirteen of
// three hundred and fifty-two and said nothing about the rest. The row is asked
// for by size instead, and the reply reports how long the list really is, so a
// list longer than the first ask is fetched again in full.
func (s *Library) Feed(feed string, limit int) (Row, error) {
	pageID, err := s.pageID(SurfaceMyNetflix)
	if err != nil {
		return Row{}, err
	}
	size := limit
	if size <= 0 {
		size = defaultFeedPageSize
	}
	section, err := s.feedSection(pageID, feed, size)
	if err != nil {
		return Row{}, err
	}
	if want := s.feedShortfall(section, limit); want > 0 {
		if section, err = s.feedSection(pageID, feed, want); err != nil {
			return Row{}, err
		}
		if got, total := len(section.Entities.Edges), section.Entities.TotalCount; got < total && limit <= 0 {
			s.client.logf("netflix returned %d of the %d titles in %q", got, total, feed)
		}
	}
	row := section.row()
	if limit > 0 && len(row.Titles) > limit {
		row.Titles = row.Titles[:limit]
	}
	return row, nil
}

// feedShortfall reports how many titles to ask for when the first reply held
// fewer than the list does, or 0 when there is nothing more worth asking for.
func (s *Library) feedShortfall(section pinotSection, limit int) int {
	got, total := len(section.Entities.Edges), section.Entities.TotalCount
	if got >= total {
		return 0
	}
	if limit > 0 {
		if got >= limit {
			return 0
		}
		if limit < total {
			return limit
		}
	}
	return total
}

// feedSection asks for the My Netflix row that is this feed, with carouselSize
// titles in it. Netflix occasionally renders the page without one of these
// rows, so a miss is asked again once: a single retry has been enough every
// time it has been observed.
func (s *Library) feedSection(pageID, feed string, carouselSize int) (pinotSection, error) {
	const attempts = 2
	for attempt := 1; attempt <= attempts; attempt++ {
		page, err := s.fetchSections(pageID, "", carouselSize)
		if err != nil {
			return pinotSection{}, err
		}
		for _, edge := range page.Page.Sections.Edges {
			if edge.Node.feed() == feed {
				return edge.Node, nil
			}
		}
		if attempt < attempts {
			s.client.logf("netflix rendered My Netflix without the %q row — asking again", feed)
		}
	}
	return pinotSection{}, fmt.Errorf("netflix did not render a %q row for this profile", feed)
}
