package client

// Netflix builds every catalogue surface — home, genre, My List, search — as a
// "Pinot" page: sections of entity cards. One response shape therefore serves
// browse and search alike; only the sections differ.
type pinotPage struct {
	Page struct {
		Sections struct {
			Edges []struct {
				Node pinotSection `json:"node"`
			} `json:"edges"`
		} `json:"sections"`
	} `json:"page"`
}

type pinotSection struct {
	TypeName      string `json:"__typename"`
	DisplayString string `json:"displayString"`
	Entities      struct {
		Edges []struct {
			Node pinotEntity `json:"node"`
		} `json:"edges"`
		TotalCount int `json:"totalCount"`
		PageInfo   struct {
			EndCursor   string `json:"endCursor"`
			HasNextPage bool   `json:"hasNextPage"`
		} `json:"pageInfo"`
	} `json:"entities"`
}

type pinotEntity struct {
	TypeName      string `json:"__typename"`
	DisplayString string `json:"displayString"`
	UnifiedEntity struct {
		TypeName        string `json:"__typename"`
		VideoID         int    `json:"videoId"`
		ContentAdvisory struct {
			MaturityLevel int `json:"maturityLevel"`
		} `json:"contentAdvisory"`
	} `json:"unifiedEntity"`
	ContextualArtwork struct {
		Artwork struct {
			URL string `json:"url"`
		} `json:"artwork"`
	} `json:"contextualArtwork"`
}

func (e pinotEntity) toTitle() (Title, bool) {
	id := e.UnifiedEntity.VideoID
	if id == 0 {
		return Title{}, false
	}
	return Title{
		ID:            id,
		Title:         e.DisplayString,
		Kind:          e.UnifiedEntity.TypeName,
		URL:           TitleURL(id),
		MaturityLevel: e.UnifiedEntity.ContentAdvisory.MaturityLevel,
		Artwork:       e.ContextualArtwork.Artwork.URL,
	}, true
}

// titles flattens one section, dropping cards that carry no video id (headers,
// autocomplete suggestions, games without a video entity).
func (s pinotSection) titles() []Title {
	out := make([]Title, 0, len(s.Entities.Edges))
	for _, edge := range s.Entities.Edges {
		if title, ok := edge.Node.toTitle(); ok {
			out = append(out, title)
		}
	}
	return out
}

// rows returns every non-empty section of the page, in page order.
func (p pinotPage) rows() []Row {
	var rows []Row
	for _, edge := range p.Page.Sections.Edges {
		titles := edge.Node.titles()
		if len(titles) == 0 {
			continue
		}
		rows = append(rows, Row{Name: edge.Node.DisplayString, Titles: titles})
	}
	return rows
}

// galleryTitles flattens the gallery sections of a page, de-duplicated. Search
// answers with one gallery section plus a suggestion section the CLI ignores.
func (p pinotPage) galleryTitles() []Title {
	var titles []Title
	seen := map[int]bool{}
	for _, edge := range p.Page.Sections.Edges {
		if edge.Node.TypeName != "PinotGallerySection" {
			continue
		}
		for _, title := range edge.Node.titles() {
			if seen[title.ID] {
				continue
			}
			seen[title.ID] = true
			titles = append(titles, title)
		}
	}
	return titles
}
