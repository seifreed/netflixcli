package client

import "fmt"

// maxSeasonsFetched bounds the seasons asked for in one request; no Netflix show
// comes close.
const maxSeasonsFetched = 50

// DefaultEpisodePageSize is how many episodes a season request asks for.
const DefaultEpisodePageSize = 50

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

// Seasons lists a show's seasons, oldest first.
func (s *Catalog) Seasons(showID int) ([]Season, error) {
	var resp struct {
		Videos []struct {
			TypeName string `json:"__typename"`
			VideoID  int    `json:"videoId"`
			Seasons  struct {
				Edges []struct {
					Node struct {
						VideoID  int    `json:"videoId"`
						Number   int    `json:"number"`
						Title    string `json:"title"`
						Episodes struct {
							TotalCount int `json:"totalCount"`
						} `json:"episodes"`
						ContentAdvisory struct {
							CertificationValue string `json:"certificationValue"`
						} `json:"contentAdvisory"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"seasons"`
		} `json:"videos"`
	}
	if err := s.client.GraphQL("PreviewModalEpisodeSelector", map[string]any{
		"showId":      showID,
		"seasonCount": maxSeasonsFetched,
	}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Videos) == 0 {
		return nil, fmt.Errorf("netflix has no title %d in this region", showID)
	}
	video := resp.Videos[0]
	if len(video.Seasons.Edges) == 0 {
		return nil, fmt.Errorf("title %d is a %s, not a show with seasons", showID, video.TypeName)
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
// requested; 0 asks for a full season.
func (s *Catalog) Episodes(seasonID, limit int) ([]Episode, error) {
	if limit <= 0 || limit > DefaultEpisodePageSize {
		limit = DefaultEpisodePageSize
	}
	var resp struct {
		Videos []struct {
			Episodes struct {
				Edges []struct {
					Node struct {
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
					} `json:"node"`
				} `json:"edges"`
			} `json:"episodes"`
		} `json:"videos"`
	}
	if err := s.client.GraphQL("PreviewModalEpisodeSelectorSeasonEpisodes", map[string]any{
		"seasonId":          seasonID,
		"count":             limit,
		"cursor":            nil,
		"opaqueImageFormat": "WEBP",
		"artworkContext":    map[string]any{},
	}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Videos) == 0 {
		return nil, fmt.Errorf("netflix has no season %d in this region", seasonID)
	}
	edges := resp.Videos[0].Episodes.Edges
	episodes := make([]Episode, 0, len(edges))
	for _, edge := range edges {
		node := edge.Node
		episodes = append(episodes, Episode{
			ID:          node.VideoID,
			Number:      node.Number,
			Title:       node.Title,
			Synopsis:    node.ContextualSynopsis.Text,
			RuntimeSec:  node.RuntimeSec,
			ProgressSec: node.Bookmark.Position,
			Playable:    node.IsPlayable,
			URL:         TitleURL(node.VideoID),
		})
	}
	return episodes, nil
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
