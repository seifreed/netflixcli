package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Every Netflix page ships the rows it renders as an Apollo cache embedded in
// the HTML:
//
//	netflix.reactContext.models.graphql = JSON.parse('{"data":{…}}')
//
// Reading that cache gives the browse surfaces — home, My Netflix, a genre —
// straight out of the page the browser would render, with no need to replay the
// page-assembler query the app issues for them.
var apolloCacheRe = regexp.MustCompile(`(?s)netflix\.reactContext\.models\.graphql\s*=\s*JSON\.parse\('((?:[^'\\]|\\.)*)'\)`)

// apolloCache is the normalised store: entity key → fields. Fields reference
// other entities as {"__ref": "<key>"}.
type apolloCache map[string]map[string]any

func parseApolloCache(html string) (apolloCache, error) {
	m := apolloCacheRe.FindStringSubmatch(html)
	if m == nil {
		return nil, fmt.Errorf("this Netflix page carries no row data (not signed in, or the page layout changed)")
	}
	var envelope struct {
		Data apolloCache `json:"data"`
	}
	if err := json.Unmarshal([]byte(unescapeJSString(m[1])), &envelope); err != nil {
		return nil, fmt.Errorf("parse netflix page data: %w", err)
	}
	if len(envelope.Data) == 0 {
		return nil, fmt.Errorf("netflix page data is empty")
	}
	return envelope.Data, nil
}

// unescapeJSString resolves the escapes of a single-quoted JavaScript string
// literal, so what is left is the JSON Netflix wrapped in it. Only the escapes
// a JSON payload can produce are undone: a backslash, a quote closing the
// literal, and a quote the serialiser escaped anyway.
func unescapeJSString(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '\\' || s[i+1] == '\'' || s[i+1] == '"') {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// deref resolves a cache reference to the entity it points at; a plain object is
// returned as is.
func (c apolloCache) deref(v any) map[string]any {
	obj, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	ref, ok := obj["__ref"].(string)
	if !ok {
		return obj
	}
	return c[ref]
}

// field reads a cache field by name. Apollo keys a field that takes arguments as
// `name({…})`, so an exact match is tried first and a prefix match second.
func field(obj map[string]any, name string) any {
	if obj == nil {
		return nil
	}
	if v, ok := obj[name]; ok {
		return v
	}
	for k, v := range obj {
		if strings.HasPrefix(k, name+"(") {
			return v
		}
	}
	return nil
}

func fieldString(obj map[string]any, name string) string {
	s, _ := field(obj, name).(string)
	return s
}

func fieldInt(obj map[string]any, name string) int {
	switch v := field(obj, name).(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// rootPage finds the browse page the cache was built for.
func (c apolloCache) rootPage() map[string]any {
	return c.deref(field(c["ROOT_QUERY"], "pinotBrowsePage"))
}

// knownFeeds are the row identities Netflix encodes in a section's page-update
// actions. The row titles themselves are localised, so these are what the CLI
// matches on.
var knownFeeds = []string{"continuewatching", "playlist", "favoritetitles", "reminders", "trailers"}

// sectionFeed reports which personal feed a section is, or "" for an editorial
// row. The feed name is carried inside the base64 ids of the section's
// page-update actions.
func (c apolloCache) sectionFeed(section map[string]any) string {
	listeners, _ := field(section, "eventListeners").([]any)
	for _, listener := range listeners {
		obj, ok := listener.(map[string]any)
		if !ok {
			continue
		}
		actions, _ := obj["actions"].([]any)
		for _, action := range actions {
			actionObj, ok := action.(map[string]any)
			if !ok {
				continue
			}
			ref, _ := actionObj["__ref"].(string)
			_, encoded, found := strings.Cut(ref, ":")
			if !found {
				continue
			}
			decoded := decodeBase64Loose(encoded)
			for _, feed := range knownFeeds {
				if strings.Contains(decoded, feed) {
					return feed
				}
			}
		}
	}
	return ""
}

// decodeBase64Loose decodes ids that may be standard or URL-safe base64 and may
// have lost their padding. Undecodable input yields "" rather than an error:
// the caller is only sniffing for a marker.
func decodeBase64Loose(s string) string {
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(raw)
	}
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(raw)
}

// rows walks the page's sections into the CLI's row model. A personal row is
// kept even when it is empty — "My List is empty" is an answer, and dropping it
// would be indistinguishable from Netflix not rendering it at all — while an
// empty editorial row is just noise.
func (c apolloCache) rows() []Row {
	page := c.rootPage()
	if page == nil {
		return nil
	}
	edges, _ := field(c.deref(field(page, "sections")), "edges").([]any)
	rows := make([]Row, 0, len(edges))
	for _, edge := range edges {
		section := c.deref(field(c.deref(edge), "node"))
		if section == nil {
			continue
		}
		titles := c.sectionTitles(section)
		feed := c.sectionFeed(section)
		if len(titles) == 0 && feed == "" {
			continue
		}
		rows = append(rows, Row{
			Name:   fieldString(section, "displayString"),
			Feed:   feed,
			Ranked: c.sectionRanked(section),
			Titles: titles,
		})
	}
	return rows
}

// sectionRanked reports whether the cached section is a ranking, by the same
// card treatment the fetched sections carry.
func (c apolloCache) sectionRanked(section map[string]any) bool {
	edges, _ := field(c.deref(field(section, "entities")), "edges").([]any)
	for _, edge := range edges {
		card := c.deref(field(c.deref(edge), "node"))
		if fieldString(card, "__typename") == rankedTreatment {
			return true
		}
	}
	return false
}

func (c apolloCache) sectionTitles(section map[string]any) []Title {
	edges, _ := field(c.deref(field(section, "entities")), "edges").([]any)
	titles := make([]Title, 0, len(edges))
	for _, edge := range edges {
		card := c.deref(field(c.deref(edge), "node"))
		entity := c.deref(field(card, "unifiedEntity"))
		id := fieldInt(entity, "videoId")
		if id == 0 {
			continue
		}
		name := fieldString(card, "displayString")
		if name == "" {
			name = fieldString(entity, "title")
		}
		titles = append(titles, Title{
			ID:            id,
			Title:         name,
			Kind:          fieldString(entity, "__typename"),
			URL:           TitleURL(id),
			MaturityLevel: fieldInt(c.deref(field(entity, "contentAdvisory")), "maturityLevel"),
			Artwork:       fieldString(c.deref(field(c.deref(field(card, "contextualArtwork")), "artwork")), "url"),
		})
	}
	return titles
}
