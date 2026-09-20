package client

import (
	"fmt"
	"strings"
)

// EntityState is what a write returns: the title as Netflix sees it afterwards.
type EntityState struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	InMyList    bool   `json:"inMyList"`
	ThumbRating string `json:"thumbRating,omitempty"`
	URL         string `json:"url"`
}

type entityEnvelope struct {
	Entity struct {
		VideoID      int    `json:"videoId"`
		Title        string `json:"title"`
		IsInPlaylist bool   `json:"isInPlaylist"`
		ThumbRating  string `json:"thumbRating"`
	} `json:"entity"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (e entityEnvelope) state() (EntityState, error) {
	if len(e.Errors) > 0 {
		return EntityState{}, fmt.Errorf("netflix refused the change: %s", e.Errors[0].Message)
	}
	return EntityState{
		ID:          e.Entity.VideoID,
		Title:       e.Entity.Title,
		InMyList:    e.Entity.IsInPlaylist,
		ThumbRating: e.Entity.ThumbRating,
		URL:         TitleURL(e.Entity.VideoID),
	}, nil
}

// AddToMyList saves a title to the current profile's My List.
func (c *Client) AddToMyList(videoID int) (EntityState, error) {
	return c.playlistMutation("AddToPlaylist", "addEntityToPlaylist", videoID)
}

// RemoveFromMyList drops a title from the current profile's My List.
func (c *Client) RemoveFromMyList(videoID int) (EntityState, error) {
	return c.playlistMutation("RemoveFromPlaylist", "removeEntityFromPlaylist", videoID)
}

func (c *Client) playlistMutation(op, field string, videoID int) (EntityState, error) {
	var resp map[string]entityEnvelope
	if err := c.GraphQL(op, map[string]any{"entityId": entityID(videoID)}, &resp); err != nil {
		return EntityState{}, err
	}
	result, ok := resp[field]
	if !ok {
		return EntityState{}, fmt.Errorf("netflix returned no result for %s", op)
	}
	return result.state()
}

// ThumbRatings maps the CLI's rating words to Netflix's enum.
var ThumbRatings = map[string]string{
	"up":      "THUMBS_UP",
	"down":    "THUMBS_DOWN",
	"love":    "THUMBS_WAY_UP",
	"way-up":  "THUMBS_WAY_UP",
	"none":    "THUMBS_UNRATED",
	"unrated": "THUMBS_UNRATED",
}

// ParseThumbRating maps a rating word to the enum Netflix expects.
func ParseThumbRating(word string) (string, error) {
	rating, ok := ThumbRatings[strings.ToLower(strings.TrimSpace(word))]
	if !ok {
		return "", fmt.Errorf("unknown rating %q (want up, down, love or none)", word)
	}
	return rating, nil
}

// Rate sets the current profile's thumb rating for a title.
func (c *Client) Rate(videoID int, rating string) (EntityState, error) {
	enum, err := ParseThumbRating(rating)
	if err != nil {
		return EntityState{}, err
	}
	var resp struct {
		SetEntityThumbRating entityEnvelope `json:"setEntityThumbRating"`
	}
	if err := c.GraphQL("SetEntityThumbRating", map[string]any{
		"entityId": entityID(videoID),
		"rating":   enum,
	}, &resp); err != nil {
		return EntityState{}, err
	}
	return resp.SetEntityThumbRating.state()
}
