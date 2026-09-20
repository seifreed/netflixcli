package client

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
)

// Viewing is one entry of a profile's viewing activity.
type Viewing struct {
	Title string `json:"title"`
	Date  string `json:"date"` // ISO 8601 (YYYY-MM-DD)
}

// History returns the profile's viewing activity, newest first. Netflix only
// exposes this as the CSV its "Download all" button produces, so the CLI parses
// that. An empty profileGUID means the profile the session is acting as.
func (c *Client) History(profileGUID string) ([]Viewing, error) {
	if strings.TrimSpace(profileGUID) == "" {
		profile, err := c.CurrentProfile()
		if err != nil {
			return nil, err
		}
		profileGUID = profile.GUID
	}
	var resp struct {
		CSV string `json:"viewingHistoryCSV"`
	}
	if err := c.GraphQL("viewingHistoryCSV", map[string]any{
		"options": map[string]any{"profileGuid": profileGUID},
	}, &resp); err != nil {
		return nil, err
	}
	return parseHistoryCSV(resp.CSV)
}

// parseHistoryCSV reads the two-column CSV Netflix serves ("Title,Date", dates
// in US M/D/YY), normalising the dates to ISO 8601 so output sorts and compares
// sensibly. A date Netflix formats differently is passed through untouched.
func parseHistoryCSV(raw string) ([]Viewing, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = -1
	var out []Viewing
	for row := 0; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("parse viewing history CSV: %w", err)
		}
		if len(record) < 2 {
			continue
		}
		if row == 0 && strings.EqualFold(strings.TrimSpace(record[0]), "title") {
			continue // header
		}
		out = append(out, Viewing{Title: record[0], Date: normalizeHistoryDate(record[1])})
	}
}

func normalizeHistoryDate(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{"1/2/06", "1/2/2006", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return raw
}
