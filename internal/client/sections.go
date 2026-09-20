package client

// A browse page server-renders only its first eight rows. The rest — including
// the Top 10 rows, which live past the twentieth — are fetched the way the page
// fetches them when the user scrolls: FetchMoreSections, paged by cursor.

import (
	"encoding/json"
	"fmt"
)

// sectionArtworkParams is the artwork projection FetchMoreSections declares.
// Every one of its variables is non-null and typed to a specific artwork kind,
// so it is sent verbatim as the web app sends it: a generic value makes the
// page assembler fail.
const sectionArtworkParams = `{"imageParamsForStandardBoxart":{"artworkType":"SDP","dimension":{"width":342,"height":192},"features":{"enableLockBadgeChecks":true,"fallbackStrategy":"STILL"}},"imageParamsForStandardBoxartHighRes":{"artworkType":"SDP","dimension":{"width":665,"height":375},"features":{"fallbackStrategy":"STILL","enableLockBadgeChecks":true}},"imageParamsForPodcastEpisodicStill":{"artworkType":"SEGMENT_STILL","dimension":{"width":342,"height":192,"scaleStrategy":"COVER"},"features":{"graybox":false}},"imageParamsForPodcastEpisodicStillHighRes":{"artworkType":"SEGMENT_STILL","dimension":{"width":665,"height":375,"scaleStrategy":"COVER"},"features":{"graybox":false}},"imageParamsForPodcastEpisodicLogo":{"artworkType":"LOGO_HORIZONTAL_CROPPED","dimension":{"width":800,"height":126,"scaleStrategy":"CONTAIN"},"features":{"tone":"LIGHT"}},"imageParamsForRankedBoxart":{"artworkType":"BOXSHOT","dimension":{"width":426,"height":607},"features":{"fallbackStrategy":"STILL","suppressTop10Badge":true}},"imageParamsForContinueWatchingBoxart":{"artworkType":"SDP","dimension":{"width":342,"height":192},"features":{"fallbackStrategy":"STILL"}},"imageParamsForContinueWatchingBoxartHighRes":{"artworkType":"SDP","dimension":{"width":665,"height":375},"features":{"fallbackStrategy":"STILL"}},"imageParamsForMobileGameBoxart":{"artworkType":"APP_ICON","dimension":{"width":200,"height":200},"formats":["WEBP","JPG","PNG"]},"imageParamsForCloudGameBoxart":{"artworkType":"SDP","dimension":{"width":342,"height":192},"features":{"fallbackStrategy":"STILL"}},"imageParamsForCloudGameBoxartHighRes":{"artworkType":"SDP","dimension":{"width":665,"height":375},"features":{"fallbackStrategy":"STILL"}},"imageParamsForBillboardLogo":{"artworkType":"LOGO_STACKED_CROPPED","dimension":{"height":260,"width":650},"formats":["WEBP","JPG","PNG"]},"imageParamsForBrandLogo":{"artworkType":"BRAND_LOGO_SMALL_FLEX","formats":["WEBP","JPG","PNG"],"dimension":{"height":30},"features":{"graybox":false,"tone":"LIGHT"}},"imageParamsForAppIcon":{"artworkType":"APP_ICON","dimension":{"width":200,"height":200},"formats":["WEBP","JPG","PNG"]},"imageParamsForHorizontalBillboardBackground":{"artworkType":"ECLIPSE_BILLBOARD","formats":["WEBP","JPG","PNG"],"dimension":{"width":1920,"height":1080}},"imageParamsForCreatorHubHeaderAvatar":{"artworkType":"COLLECTION_AVATAR_CIRCLE","formats":["WEBP","JPG","PNG"],"dimension":{"width":200,"height":200}},"imageParamsForHubHeaderBackground":{"artworkType":"COLLECTION_HUB_HEADER","formats":["WEBP","JPG","PNG"],"dimension":{"width":1280,"height":400}},"imageParamsForHubHeaderMobileBackground":{"artworkType":"MLP_MOBILE_BILLBOARD","formats":["WEBP","JPG","PNG"],"dimension":{"width":640,"height":360}},"imageParamsForVerticalBillboardBackground":{"artworkType":"VERTICAL_BILLBOARD_PLUS","dimension":{"height":1000,"width":640},"formats":["WEBP","JPG","PNG"]},"imageParamsForVerticalBackgroundFallback":{"artworkType":"ECLIPSE_BOXART_BACKGROUND","formats":["WEBP","JPG","PNG"],"dimension":{"width":500}},"imageParamsForCharacterCircle":{"artworkType":"SQUAREHEADSHOT_1000x1000","dimension":{"width":200,"height":200},"formats":["WEBP","JPG","PNG"]},"imageParamsForEntryPointBackground":{"artworkType":"MLP_ENTRY_POINT_BACKGROUND","dimension":{"width":1024},"features":{"fallbackStrategy":"STILL"}},"imageParamsForEntryPointLogo":{"artworkType":"LOGO_STACKED_CROPPED","dimension":{"height":260},"formats":["WEBP","JPG","PNG"]}}`

// sectionPageSize is how many rows one request asks for; the web app asks for
// about this many.
const sectionPageSize = 20

// browseCarouselSize is how many titles each row of a browse page carries. It
// is the web app's own value: a browse row is a preview, and `browse --limit`
// trims it further. A feed asks for its whole list instead — see Feed.
const browseCarouselSize = 13

// maxSectionPages bounds the paging, so a cursor that stops advancing cannot
// loop for ever.
const maxSectionPages = 10

// optionalString sends "" as JSON null, which is what an absent cursor means.
func optionalString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func sectionVars() map[string]any {
	var vars map[string]any
	if err := json.Unmarshal([]byte(sectionArtworkParams), &vars); err != nil {
		panic("netflix: section artwork params are not valid JSON: " + err.Error())
	}
	return vars
}

// AllRows returns every row of a surface, not just the eight the page carries.
// It costs one request per twenty rows, so callers that only need the visible
// rows should use Browse.
func (s *Library) AllRows(surface string) ([]Row, error) {
	pageID, err := s.pageID(surface)
	if err != nil {
		return nil, err
	}
	var rows []Row
	cursor := ""
	for page := 0; page < maxSectionPages; page++ {
		batch, next, more, err := s.sectionPage(pageID, cursor)
		if err != nil {
			return nil, err
		}
		rows = append(rows, batch...)
		if !more || next == "" || next == cursor {
			break
		}
		cursor = next
	}
	return rows, nil
}

// Top returns the ranked rows of the home page — Netflix's top 10 lists. They
// are found by the card treatment, which names a ranking, rather than by the
// row title, which is localised.
func (s *Library) Top() ([]Row, error) {
	rows, err := s.AllRows(SurfaceHome)
	if err != nil {
		return nil, err
	}
	var ranked []Row
	for _, row := range rows {
		if row.Ranked {
			ranked = append(ranked, row)
		}
	}
	if len(ranked) == 0 {
		return nil, fmt.Errorf("netflix rendered no ranked row for this profile")
	}
	return ranked, nil
}

// pageID reads the identifier the page was assembled under; FetchMoreSections
// continues that same assembly.
func (s *Library) pageID(surface string) (string, error) {
	cache, err := s.surfaceCache(surface)
	if err != nil {
		return "", err
	}
	id := fieldString(cache.rootPage(), "id")
	if id == "" {
		return "", fmt.Errorf("netflix page carries no id to fetch more rows with")
	}
	return id, nil
}

func (s *Library) sectionPage(pageID, cursor string) (rows []Row, next string, more bool, err error) {
	page, err := s.fetchSections(pageID, cursor, browseCarouselSize)
	if err != nil {
		return nil, "", false, err
	}
	info := page.Page.Sections.PageInfo
	return page.rows(), info.EndCursor, info.HasNextPage, nil
}

// fetchSections asks the page assembler for a slice of a surface's rows.
// carouselSize is how many titles each of those rows carries.
func (s *Library) fetchSections(pageID, cursor string, carouselSize int) (pinotPage, error) {
	vars := sectionVars()
	vars["pageId"] = pageID
	vars["sectionsAfterCursor"] = optionalString(cursor)
	vars["sectionCount"] = sectionPageSize
	vars["carouselPageSize"] = carouselSize
	vars["fetchHighResCards"] = false
	vars["eddEnabled"] = false
	var page pinotPage
	if err := s.client.GraphQL("FetchMoreSections", vars, &page); err != nil {
		return pinotPage{}, err
	}
	return page, nil
}
