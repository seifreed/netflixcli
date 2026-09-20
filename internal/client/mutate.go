package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EntityState is what a write returns: the title as Netflix sees it afterwards.
type EntityState struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	InMyList    bool   `json:"inMyList"`
	Reminder    bool   `json:"reminder"`
	ThumbRating string `json:"thumbRating,omitempty"`
	URL         string `json:"url"`
}

type entityEnvelope struct {
	Entity struct {
		VideoID          int    `json:"videoId"`
		Title            string `json:"title"`
		IsInPlaylist     bool   `json:"isInPlaylist"`
		IsInRemindMeList bool   `json:"isInRemindMeList"`
		ThumbRating      string `json:"thumbRating"`
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
		Reminder:    e.Entity.IsInRemindMeList,
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

// AddReminder asks Netflix to remind this profile when a title arrives.
//
// Netflix has no reminder for a title that is already available: asking for one
// puts the title in My List instead, and says so through the flags it returns.
// The reminder mutations do not return a title, so EntityState.Title is empty.
func (c *Client) AddReminder(videoID int) (EntityState, error) {
	return c.reminderMutation("AddReminder", "addUnifiedEntityToRemindMe", videoID)
}

// RemoveReminder drops a title's release reminder.
func (c *Client) RemoveReminder(videoID int) (EntityState, error) {
	return c.reminderMutation("RemoveReminder", "removeUnifiedEntityFromRemindMe", videoID)
}

// The reminder mutations answer with the entity itself rather than wrapping it,
// so the envelope is filled from that.
func (c *Client) reminderMutation(op, field string, videoID int) (EntityState, error) {
	var resp map[string]json.RawMessage
	if err := c.GraphQL(op, map[string]any{"entityId": entityID(videoID)}, &resp); err != nil {
		return EntityState{}, err
	}
	raw, ok := resp[field]
	if !ok {
		return EntityState{}, fmt.Errorf("netflix returned no result for %s", op)
	}
	var env entityEnvelope
	if err := json.Unmarshal(raw, &env.Entity); err != nil {
		return EntityState{}, fmt.Errorf("decode %s result: %w", op, err)
	}
	if env.Entity.VideoID == 0 {
		return EntityState{}, fmt.Errorf("netflix did not accept a reminder for title %d (it may already be available)", videoID)
	}
	return env.state()
}

// RemoveFromContinueWatching drops a title from the profile's Continue Watching
// row. It does not erase the viewing history entry.
func (c *Client) RemoveFromContinueWatching(videoID int) error {
	var resp struct {
		RemoveFromContinueWatching struct {
			Success bool `json:"success"`
		} `json:"removeFromContinueWatching"`
	}
	if err := c.GraphQL("RemoveFromContinueWatching", map[string]any{
		"unifiedEntityId": entityID(videoID),
	}, &resp); err != nil {
		return err
	}
	if !resp.RemoveFromContinueWatching.Success {
		return fmt.Errorf("netflix refused to drop title %d from Continue Watching", videoID)
	}
	return nil
}
