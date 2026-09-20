package client

import (
	"sort"
	"strconv"
	"strings"
)

// Genre is one entry of the browse navigation menu. ID is the category id the
// browse surfaces take ("genre-83"); Number is the bare id Netflix's own URLs
// use.
type Genre struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// Genres lists the genres this account's region offers, sorted by title. They
// are what makes `browse <genre>` usable: Netflix never shows these ids in the
// UI.
func (c *Client) Genres() ([]Genre, error) {
	var resp struct {
		Categories []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"navigationMenuCategories"`
	}
	// The input takes no fields: the menu is decided by the session's region
	// and profile.
	if err := c.GraphQL("GetGenreSubgenres", map[string]any{"options": map[string]any{}}, &resp); err != nil {
		return nil, err
	}
	genres := make([]Genre, 0, len(resp.Categories))
	for _, category := range resp.Categories {
		if category.ID == "" {
			continue
		}
		number, err := strconv.Atoi(strings.TrimPrefix(category.ID, "genre-"))
		if err != nil {
			continue
		}
		genres = append(genres, Genre{
			ID:     category.ID,
			Number: number,
			Title:  strings.TrimSpace(category.Title),
			URL:    BaseURL + "/browse/genre/" + strconv.Itoa(number),
		})
	}
	sort.Slice(genres, func(i, j int) bool { return genres[i].Title < genres[j].Title })
	return genres, nil
}

// MatchGenres keeps the genres whose title contains the query, case-insensitively.
// An empty query keeps them all.
func MatchGenres(genres []Genre, query string) []Genre {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return genres
	}
	var out []Genre
	for _, g := range genres {
		if strings.Contains(strings.ToLower(g.Title), query) {
			out = append(out, g)
		}
	}
	return out
}
