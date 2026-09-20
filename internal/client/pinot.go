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
			PageInfo connectionPageInfo `json:"pageInfo"`
		} `json:"sections"`
	} `json:"page"`
}

// connectionPageInfo is the cursor every paged connection answers with —
// sections, search galleries and a season's episodes alike.
type connectionPageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

type pinotSection struct {
	TypeName      string `json:"__typename"`
	ID            string `json:"_id"` // the node id pagination asks for more by
	DisplayString string `json:"displayString"`
	// EventListeners carry the section's page-update actions, whose base64 ids
	// name the personal feed the section is — the same marker the page cache
	// carries (see apollo.go).
	EventListeners []struct {
		Actions []struct {
			ID string `json:"id"`
		} `json:"actions"`
	} `json:"eventListeners"`
	Entities struct {
		Edges []struct {
			Node pinotEntity `json:"node"`
		} `json:"edges"`
		PageInfo connectionPageInfo `json:"pageInfo"`
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

// feed reports which personal feed this section is, or "" for an editorial row.
func (s pinotSection) feed() string {
	for _, listener := range s.EventListeners {
		for _, action := range listener.Actions {
			if feed := feedFromActionID(action.ID); feed != "" {
				return feed
			}
		}
	}
	return ""
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

// rows returns every section of a fetched page, in page order.
//
// It keeps the same rule the page cache does: a personal row survives being
// empty, because "My List is empty" is an answer and dropping it would be
// indistinguishable from Netflix not rendering it, while an empty editorial row
// is just noise. `browse --all` used to lose those rows and every row's feed,
// so it answered with less than `browse` did for the same surface.
func (p pinotPage) rows() []Row {
	rows := make([]Row, 0, len(p.Page.Sections.Edges))
	for _, edge := range p.Page.Sections.Edges {
		titles := edge.Node.titles()
		feed := edge.Node.feed()
		if len(titles) == 0 && feed == "" {
			continue
		}
		rows = append(rows, Row{
			Name:   edge.Node.DisplayString,
			Feed:   feed,
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
