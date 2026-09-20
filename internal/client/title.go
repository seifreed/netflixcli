package client

import (
	"fmt"
	"strconv"
	"strings"
)

// Title is one catalogue entry as the CLI reports it.
type Title struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Kind          string `json:"kind"` // Movie, Show, Game, …
	URL           string `json:"url"`
	MaturityLevel int    `json:"maturityLevel,omitempty"`
	Artwork       string `json:"artwork,omitempty"`
}

// TitleURL is the canonical watch page for a Netflix video id.
func TitleURL(id int) string { return fmt.Sprintf("%s/title/%d", BaseURL, id) }

// TitleDetail is everything the CLI reports about one title.
type TitleDetail struct {
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	Kind         string   `json:"kind"`
	Year         int      `json:"year,omitempty"`
	RuntimeSec   int      `json:"runtimeSec,omitempty"`
	Synopsis     string   `json:"synopsis,omitempty"`
	Genres       []string `json:"genres,omitempty"`
	Moods        []string `json:"moods,omitempty"`
	Cast         []string `json:"cast,omitempty"`
	Directors    []string `json:"directors,omitempty"`
	Writers      []string `json:"writers,omitempty"`
	Rating       string   `json:"rating,omitempty"`
	RatingReason string   `json:"ratingReason,omitempty"`
	WatchStatus  string   `json:"watchStatus,omitempty"`
	ThumbRating  string   `json:"thumbRating,omitempty"`
	InMyList     bool     `json:"inMyList"`
	Playable     bool     `json:"playable"`
	Badges       []string `json:"badges,omitempty"`
	Similar      []int    `json:"similar,omitempty"`
	Artwork      string   `json:"artwork,omitempty"`
	URL          string   `json:"url"`
}

// Runtime renders RuntimeSec as "2h 12m" / "48m", or "" when unknown.
func (t TitleDetail) Runtime() string {
	switch {
	case t.RuntimeSec <= 0:
		return ""
	case t.RuntimeSec < 3600:
		return fmt.Sprintf("%dm", t.RuntimeSec/60)
	default:
		return fmt.Sprintf("%dh %02dm", t.RuntimeSec/3600, (t.RuntimeSec%3600)/60)
	}
}

// personConnection is the shape GraphQL uses for cast, directors and writers.
type personConnection struct {
	Edges []struct {
		Node struct {
			Name string `json:"name"`
		} `json:"node"`
	} `json:"edges"`
}

func (p personConnection) names() []string {
	out := make([]string, 0, len(p.Edges))
	for _, e := range p.Edges {
		if e.Node.Name != "" {
			out = append(out, e.Node.Name)
		}
	}
	return out
}

type detailEntity struct {
	TypeName        string   `json:"__typename"`
	VideoID         int      `json:"videoId"`
	Title           string   `json:"title"`
	LatestYear      int      `json:"latestYear"`
	RuntimeSec      int      `json:"runtimeSec"`
	IsPlayable      bool     `json:"isPlayable"`
	IsInPlaylist    bool     `json:"isInPlaylist"`
	WatchStatus     string   `json:"watchStatus"`
	ThumbRating     string   `json:"thumbRating"`
	PlaybackBadges  []string `json:"playbackBadges"`
	ContextualSynop struct {
		Text string `json:"text"`
	} `json:"contextualSynopsis"`
	ContentAdvisory struct {
		CertificationValue  string `json:"certificationValue"`
		MaturityDescription string `json:"maturityDescription"`
	} `json:"contentAdvisory"`
	GenreTags struct {
		Edges []struct {
			Node struct {
				Name string `json:"name"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"genreTags"`
	MoodTags []struct {
		DisplayName string `json:"displayName"`
	} `json:"moodTags"`
	Cast      personConnection `json:"cast"`
	Directors personConnection `json:"directors"`
	Writers   personConnection `json:"writers"`
	Similars  []struct {
		VideoID int `json:"videoId"`
	} `json:"similars"`
	Boxart struct {
		URL string `json:"url"`
	} `json:"boxart"`
}

func (e detailEntity) toDetail() TitleDetail {
	d := TitleDetail{
		ID:           e.VideoID,
		Title:        e.Title,
		Kind:         e.TypeName,
		Year:         e.LatestYear,
		RuntimeSec:   e.RuntimeSec,
		Synopsis:     e.ContextualSynop.Text,
		Moods:        make([]string, 0, len(e.MoodTags)),
		Cast:         e.Cast.names(),
		Directors:    e.Directors.names(),
		Writers:      e.Writers.names(),
		Rating:       e.ContentAdvisory.CertificationValue,
		RatingReason: e.ContentAdvisory.MaturityDescription,
		WatchStatus:  e.WatchStatus,
		ThumbRating:  e.ThumbRating,
		InMyList:     e.IsInPlaylist,
		Playable:     e.IsPlayable,
		Badges:       e.PlaybackBadges,
		Artwork:      e.Boxart.URL,
		URL:          TitleURL(e.VideoID),
	}
	for _, g := range e.GenreTags.Edges {
		if g.Node.Name != "" {
			d.Genres = append(d.Genres, g.Node.Name)
		}
	}
	for _, m := range e.MoodTags {
		if m.DisplayName != "" {
			d.Moods = append(d.Moods, m.DisplayName)
		}
	}
	for _, s := range e.Similars {
		if s.VideoID != 0 {
			d.Similar = append(d.Similar, s.VideoID)
		}
	}
	return d
}

// entityID renders a Netflix video id as the unified entity id GraphQL expects.
func entityID(videoID int) string { return "Video:" + strconv.Itoa(videoID) }

// ParseTitleID accepts a bare video id or any netflix.com URL/path that carries
// one (/title/70095139, /watch/70095139?…).
func ParseTitleID(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty title id")
	}
	if id, err := strconv.Atoi(raw); err == nil && id > 0 {
		return id, nil
	}
	trimmed := raw
	if i := strings.IndexAny(trimmed, "?#"); i >= 0 {
		trimmed = trimmed[:i]
	}
	trimmed = strings.TrimSuffix(trimmed, "/")
	if i := strings.LastIndexByte(trimmed, '/'); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	if id, err := strconv.Atoi(trimmed); err == nil && id > 0 {
		return id, nil
	}
	return 0, fmt.Errorf("%q is not a Netflix title id or /title/<id> URL", raw)
}

// Detail fetches one title's full record, the same query the web app runs when
// a title page opens.
func (s *Catalog) Detail(videoID int) (TitleDetail, error) {
	vars := detailImageVars("ODP")
	vars["unifiedEntityId"] = entityID(videoID)
	vars["videoId"] = videoID
	vars["checkLinearChannel"] = true
	var resp struct {
		UnifiedEntities []detailEntity `json:"unifiedEntities"`
	}
	if err := s.client.GraphQL("DetailModal", vars, &resp); err != nil {
		if isNotFound(err) {
			return TitleDetail{}, fmt.Errorf("netflix has no title %d in this region", videoID)
		}
		return TitleDetail{}, err
	}
	if len(resp.UnifiedEntities) == 0 {
		return TitleDetail{}, fmt.Errorf("netflix has no title %d in this region", videoID)
	}
	return resp.UnifiedEntities[0].toDetail(), nil
}

// Details fetches several titles in one request, the way the web app populates
// a row of cards. Titles Netflix does not return are silently absent.
func (s *Catalog) Details(videoIDs []int) ([]TitleDetail, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(videoIDs))
	for _, id := range videoIDs {
		ids = append(ids, entityID(id))
	}
	vars := detailImageVars("BOB")
	vars["unifiedEntityIds"] = ids
	var resp struct {
		UnifiedEntities []detailEntity `json:"unifiedEntities"`
	}
	if err := s.client.GraphQL("MiniModalQuery", vars, &resp); err != nil {
		return nil, err
	}
	out := make([]TitleDetail, 0, len(resp.UnifiedEntities))
	for _, e := range resp.UnifiedEntities {
		if e.VideoID != 0 {
			out = append(out, e.toDetail())
		}
	}
	return out, nil
}

// detailImageVars are the artwork and merchandising variables both detail
// queries require. evidenceContext is "ODP" for a title page, "BOB" for a card.
func detailImageVars(evidenceContext string) map[string]any {
	return map[string]any{
		"opaqueImageFormat":       "WEBP",
		"transparentImageFormat":  "WEBP",
		"videoMerchEnabled":       true,
		"fetchPromoVideoOverride": false,
		"hasPromoVideoOverride":   false,
		"promoVideoId":            0,
		"videoMerchContext":       "BROWSE",
		"isLiveEpisodic":          false,
		"includeCroppedLogo":      false,
		"artworkContext":          map[string]any{},
		"textEvidenceUiContext":   evidenceContext,
	}
}
