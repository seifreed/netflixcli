package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// searchPageCapabilities is the capability document the search query requires:
// it tells the Pinot page builder which section and card shapes the client can
// render. It is sent verbatim, exactly as the web app sends it.
const searchPageCapabilities = `{"base":{"canHandlePlayingCloudGames":true,"capabilitiesBySection":{"pinotGallery":{"base":{"capabilitiesBySectionTreatment":{"pinotCreatorHome":{"base":{"capabilitiesByEntityTreatment":{"pinotStandardBoxshot":{"base":{"canHandleEntityKinds":["MOVIE","SHOW","EPISODE","SEASON","SUPPLEMENTAL"]}},"pinotStandardCloudAppIcon":{"base":{"canHandleEntityKinds":["GAME"]}},"pinotStandardMobileAppIcon":{"base":{"canHandleEntityKinds":["GAME"]}},"pinotStandardDestination":{"base":{"canHandleEntityKinds":["GENERIC_CONTAINER"]}}},"maxTotalEntities":300}},"pinotStandard":{"base":{"capabilitiesByEntityTreatment":{"pinotStandardBoxshot":{"base":{"canHandleEntityKinds":["MOVIE","SHOW","EPISODE","SEASON","SUPPLEMENTAL"]}},"pinotStandardCloudAppIcon":{"base":{"canHandleEntityKinds":["GAME"]}},"pinotStandardMobileAppIcon":{"base":{"canHandleEntityKinds":["GAME"]}},"pinotStandardDestination":{"base":{"canHandleEntityKinds":["GENERIC_CONTAINER"]}}},"maxTotalEntities":300}}}}},"pinotList":{"base":{"capabilitiesBySectionTreatment":{"pinotSuggestions":{"base":{"capabilitiesByEntityTreatment":{"pinotSuggestion":{"base":{"canHandleEntityKinds":["AUTOCOMPLETE","MOVIE","SHOW","EPISODE","SEASON","SUPPLEMENTAL","CHARACTER","GENERIC_CONTAINER","GENRE","PERSON"]}}},"maxTotalEntities":100}}}}}},"maxTotalSections":2},"canHandleComplexSectionId":true,"canSupportPreLaunchGames":true}`

// artworkParams is the artwork projection the web app asks for. The gateway
// requires every one of these variables, even for a query that only reads the
// standard boxshot.
func artworkParams() map[string]any {
	card := func(kind string, w, h int, features string) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(
			`{"artworkType":%q,"dimension":{"width":%d,"height":%d},"features":%s}`, kind, w, h, features))
	}
	const (
		boxshot  = `{"enableLockBadgeChecks":true,"fallbackStrategy":"STILL"}`
		gameIcon = `{"fallbackStrategy":"STILL","topContentTypeBadge":true}`
	)
	return map[string]any{
		"imageParamsForStandardBoxart":         card("SDP", 342, 192, boxshot),
		"imageParamsForCloudGameBoxart":        card("GAME_CLOUD_BOXART_HORIZONTAL", 342, 192, gameIcon),
		"imageParamsForMobileGameBoxart":       card("GAME_ICON_BOXART_HORIZONTAL_CARD", 342, 192, gameIcon),
		"imageParamsForStandardBoxartHighRes":  card("SDP", 665, 375, boxshot),
		"imageParamsForCloudGameBoxartHighRes": card("GAME_CLOUD_BOXART_HORIZONTAL", 665, 375, gameIcon),
	}
}

// DefaultSearchPageSize matches the web app's own page size.
const DefaultSearchPageSize = 48

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

// Search queries the catalogue the way the web app's search page does, paging
// the result gallery until it has limit titles or Netflix runs out. limit 0
// returns the first page.
func (s *Catalog) Search(query string, limit int) ([]Title, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search needs a query")
	}
	pageSize := limit
	if pageSize <= 0 || pageSize > DefaultSearchPageSize {
		pageSize = DefaultSearchPageSize
	}
	vars := artworkParams()
	vars["searchTerm"] = query
	vars["pageSize"] = pageSize
	vars["endCursor"] = nil
	vars["eddEnabled"] = false
	vars["fetchHighResCards"] = false
	vars["options"] = map[string]any{
		"pageCapabilities": json.RawMessage(searchPageCapabilities),
		"session":          map[string]any{"id": newUUID()},
	}
	var page pinotPage
	if err := s.client.GraphQL("SearchPageQueryResults", vars, &page); err != nil {
		return nil, err
	}
	gallery, ok := page.gallerySection()
	if !ok {
		return nil, nil
	}

	var titles []Title
	seen := map[int]bool{}
	appendPage := func(section pinotSection) int {
		added := 0
		for _, title := range section.titles() {
			if seen[title.ID] {
				continue
			}
			seen[title.ID] = true
			titles = append(titles, title)
			added++
		}
		return added
	}
	appendPage(gallery)

	for limit > len(titles) && gallery.Entities.PageInfo.HasNextPage && gallery.ID != "" {
		next, err := s.searchPage(gallery.ID, gallery.Entities.PageInfo.EndCursor, pageSize)
		if err != nil {
			return nil, err
		}
		// A page that adds nothing new would otherwise loop for ever.
		if appendPage(next) == 0 {
			break
		}
		gallery = next
	}
	if limit > 0 && len(titles) > limit {
		titles = titles[:limit]
	}
	return titles, nil
}

// searchPage fetches the next slice of a result gallery, the way the search
// page does when the user scrolls to the end of it.
func (s *Catalog) searchPage(galleryID, cursor string, pageSize int) (pinotSection, error) {
	vars := artworkParams()
	vars["galleryId"] = galleryID
	vars["endCursor"] = cursor
	vars["pageSize"] = pageSize
	vars["eddEnabled"] = false
	vars["fetchHighResCards"] = false
	var resp struct {
		Node pinotSection `json:"node"`
	}
	if err := s.client.GraphQL("FetchMoreSearchGalleryItems", vars, &resp); err != nil {
		return pinotSection{}, err
	}
	return resp.Node, nil
}
