package client

import (
	"fmt"
	"strings"
)

// maxSeasonsFetched bounds the seasons asked for in one request; no Netflix show
// comes close.
const maxSeasonsFetched = 50

// DefaultEpisodePageSize is how many episodes one request asks for. A season
// with more than that is paged, the way the web app pages it when the episode
// list is scrolled.
const DefaultEpisodePageSize = 50

// maxEpisodePages bounds the paging, so a cursor that stops advancing cannot
// loop for ever. At fifty an episode this reaches a thousand-episode season.
const maxEpisodePages = 20

// Season is one season of a show.
type Season struct {
	ID       int    `json:"id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Episodes int    `json:"episodes"`
	Rating   string `json:"rating,omitempty"`
}

// Episode is one episode of a season.
type Episode struct {
	ID          int    `json:"id"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Synopsis    string `json:"synopsis,omitempty"`
	RuntimeSec  int    `json:"runtimeSec,omitempty"`
	ProgressSec int    `json:"progressSec,omitempty"`
	Playable    bool   `json:"playable"`
	URL         string `json:"url"`
}

// Runtime renders RuntimeSec as "48m" / "1h 02m", or "" when unknown.
func (e Episode) Runtime() string {
	return TitleDetail{RuntimeSec: e.RuntimeSec}.Runtime()
}

// The GraphQL shapes the episode selector answers with. They are named so the
// operations read as a translation from Netflix's wire format to the CLI's
// model, rather than burying one inside the other.
type seasonsResponse struct {
	Videos []struct {
		TypeName string `json:"__typename"`
		VideoID  int    `json:"videoId"`
		Seasons  struct {
			Edges []struct {
				Node seasonNode `json:"node"`
			} `json:"edges"`
		} `json:"seasons"`
	} `json:"videos"`
}

type seasonNode struct {
	VideoID  int    `json:"videoId"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Episodes struct {
		TotalCount int `json:"totalCount"`
	} `json:"episodes"`
	ContentAdvisory struct {
		CertificationValue string `json:"certificationValue"`
	} `json:"contentAdvisory"`
}

type episodeConnection struct {
	Edges []struct {
		Node episodeNode `json:"node"`
	} `json:"edges"`
	PageInfo connectionPageInfo `json:"pageInfo"`
}

type episodesResponse struct {
	Videos []struct {
		Episodes episodeConnection `json:"episodes"`
	} `json:"videos"`
}

type episodeNode struct {
	VideoID            int    `json:"videoId"`
	Number             int    `json:"number"`
	Title              string `json:"title"`
	RuntimeSec         int    `json:"runtimeSec"`
	IsPlayable         bool   `json:"isPlayable"`
	ContextualSynopsis struct {
		Text string `json:"text"`
	} `json:"contextualSynopsis"`
	Bookmark struct {
		Position int `json:"position"`
	} `json:"bookmark"`
}

func (n episodeNode) toEpisode() Episode {
	return Episode{
		ID:          n.VideoID,
		Number:      n.Number,
		Title:       n.Title,
		Synopsis:    n.ContextualSynopsis.Text,
		RuntimeSec:  n.RuntimeSec,
		ProgressSec: n.Bookmark.Position,
		Playable:    n.IsPlayable,
		URL:         TitleURL(n.VideoID),
	}
}

// Seasons lists a show's seasons, oldest first.
func (s *Catalog) Seasons(showID int) ([]Season, error) {
	var resp seasonsResponse
	if err := s.client.GraphQL("PreviewModalEpisodeSelector", map[string]any{
		"showId":      showID,
		"seasonCount": maxSeasonsFetched,
	}, &resp); err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("netflix has no title %d in this region", showID)
		}
		return nil, err
	}
	// A title that does not exist comes back as an entry with nothing in it, so
	// it has to be told from a real title that simply has no seasons — which
	// used to read "title 999999999 is a , not a show with seasons".
	if len(resp.Videos) == 0 || resp.Videos[0].VideoID == 0 {
		return nil, fmt.Errorf("netflix has no title %d in this region", showID)
	}
	video := resp.Videos[0]
	if len(video.Seasons.Edges) == 0 {
		kind := strings.ToLower(video.TypeName)
		if kind == "" {
			kind = "not a show"
		} else {
			kind = "a " + kind
		}
		return nil, fmt.Errorf("title %d is %s, so it has no seasons", showID, kind)
	}
	seasons := make([]Season, 0, len(video.Seasons.Edges))
	for i, edge := range video.Seasons.Edges {
		number := edge.Node.Number
		if number == 0 {
			number = i + 1 // the selector omits the number for single-season shows
		}
		seasons = append(seasons, Season{
			ID:       edge.Node.VideoID,
			Number:   number,
			Title:    edge.Node.Title,
			Episodes: edge.Node.Episodes.TotalCount,
			Rating:   edge.Node.ContentAdvisory.CertificationValue,
		})
	}
	return seasons, nil
}

// Episodes lists one season's episodes, oldest first. limit bounds how many are
// returned; 0 asks for the whole season, however long it is.
//
// One request answers with at most DefaultEpisodePageSize, so a longer season
// is paged. It used to be truncated there instead, silently: a season Netflix
// says has 63 episodes came back with 50, and an explicit --limit above the
// page size was clamped to it.
func (s *Catalog) Episodes(seasonID, limit int) ([]Episode, error) {
	var episodes []Episode
	cursor := ""
	for page := 0; page < maxEpisodePages; page++ {
		count := DefaultEpisodePageSize
		if remaining := limit - len(episodes); limit > 0 && remaining < count {
			count = remaining
		}
		conn, err := s.episodePage(seasonID, cursor, count)
		if err != nil {
			return nil, err
		}
		for _, edge := range conn.Edges {
			episodes = append(episodes, edge.Node.toEpisode())
		}
		if limit > 0 && len(episodes) >= limit {
			return episodes[:limit], nil
		}
		next := conn.PageInfo.EndCursor
		if !conn.PageInfo.HasNextPage || next == "" || next == cursor || len(conn.Edges) == 0 {
			break
		}
		cursor = next
	}
	return episodes, nil
}

// episodePage fetches one slice of a season, the way the episode list does when
// it is scrolled to the end.
func (s *Catalog) episodePage(seasonID int, cursor string, count int) (episodeConnection, error) {
	var resp episodesResponse
	if err := s.client.GraphQL("PreviewModalEpisodeSelectorSeasonEpisodes", map[string]any{
		"seasonId":          seasonID,
		"count":             count,
		"cursor":            optionalString(cursor),
		"opaqueImageFormat": "WEBP",
		"artworkContext":    map[string]any{},
	}, &resp); err != nil {
		return episodeConnection{}, err
	}
	if len(resp.Videos) == 0 {
		return episodeConnection{}, fmt.Errorf("netflix has no season %d in this region", seasonID)
	}
	return resp.Videos[0].Episodes, nil
}

// SeasonByNumber picks a show's season by its number.
func SeasonByNumber(seasons []Season, number int) (Season, error) {
	for _, s := range seasons {
		if s.Number == number {
			return s, nil
		}
	}
	return Season{}, fmt.Errorf("this show has no season %d (it has %d)", number, len(seasons))
}
