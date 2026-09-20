package client

// Netflix builds every catalogue surface — home, genre, My List, search — as a
// "Pinot" page: sections of entity cards. Browse surfaces are read from the
// page's own cache (see apollo.go); search has no such page, so its response is
// decoded here.
type pinotPage struct {
	Page struct {
		Sections struct {
			Edges []struct {
				Node pinotSection `json:"node"`
			} `json:"edges"`
			PageInfo pinotPageInfo `json:"pageInfo"`
		} `json:"sections"`
	} `json:"page"`
}

type pinotPageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type pinotSection struct {
	TypeName      string `json:"__typename"`
	ID            string `json:"_id"` // the node id pagination asks for more by
	DisplayString string `json:"displayString"`
	Entities      struct {
		Edges []struct {
			Node pinotEntity `json:"node"`
		} `json:"edges"`
		TotalCount int           `json:"totalCount"`
		PageInfo   pinotPageInfo `json:"pageInfo"`
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

// rankedTreatment is the card shape Netflix uses for a top-10 row. The titles
// of those rows are localised, so this is what identifies a ranking.
const rankedTreatment = "PinotRankedBoxshotEntityTreatment"

// ranked reports whether the section's order is a ranking.
func (s pinotSection) ranked() bool {
	for _, edge := range s.Entities.Edges {
		if edge.Node.TypeName == rankedTreatment {
			return true
		}
	}
	return false
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

// rows returns every section of a fetched page, in page order. Browse surfaces
// read their rows from the page cache instead; this walks the ones that arrive
// over GraphQL, past the eighth.
func (p pinotPage) rows() []Row {
	rows := make([]Row, 0, len(p.Page.Sections.Edges))
	for _, edge := range p.Page.Sections.Edges {
		titles := edge.Node.titles()
		if len(titles) == 0 {
			continue
		}
		rows = append(rows, Row{
			Name:   edge.Node.DisplayString,
			Ranked: edge.Node.ranked(),
			Titles: titles,
		})
	}
	return rows
}

// gallerySection returns the page's gallery of results. Search answers with one
// gallery section plus a suggestion section the CLI ignores.
func (p pinotPage) gallerySection() (pinotSection, bool) {
	for _, edge := range p.Page.Sections.Edges {
		if edge.Node.TypeName == "PinotGallerySection" {
			return edge.Node, true
		}
	}
	return pinotSection{}, false
}
