package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdBrowse prints the rows of a browse surface — the home page by default.
func cmdBrowse(args []string) error {
	fs, cf := newCommonFlags("browse")
	surface := fs.String("surface", "", "home, my-netflix, or a genre id")
	limit := fs.Int("limit", 0, "max titles per row (0 = whatever the page carries)")
	parseFlags(fs, args)
	if positional := strings.TrimSpace(strings.Join(fs.Args(), " ")); positional != "" {
		*surface = positional
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	rows, err := cl.Browse(*surface)
	if err != nil {
		return err
	}
	rows = capRows(rows, *limit)
	return output(cf, rows, func() {
		if len(rows) == 0 {
			fmt.Println("(no rows — the surface may not exist in this region)")
			return
		}
		printRows(rows)
	})
}

// cmdFeed prints one personal row of the My Netflix page: My List, Continue
// Watching, liked titles, reminders or watched trailers.
func cmdFeed(feed string) func([]string) error {
	return func(args []string) error {
		fs, cf := newCommonFlags(feed)
		limit := fs.Int("limit", 0, "max titles to return")
		parseFlags(fs, args)
		cl, err := newClient(cf)
		if err != nil {
			return err
		}
		row, err := cl.Feed(feed)
		if err != nil {
			return err
		}
		titles := capTitles(row.Titles, *limit)
		return output(cf, titles, func() {
			if len(titles) == 0 {
				fmt.Printf("(%s is empty for this profile)\n", row.Name)
				return
			}
			printTitles(titles)
		})
	}
}

func printRows(rows []client.Row) {
	for _, row := range rows {
		fmt.Printf("\n%s\n", row.Name)
		if len(row.Titles) == 0 {
			fmt.Println("  (empty)")
		}
		for _, t := range row.Titles {
			fmt.Printf("  • %s  [%d]\n", t.Title, t.ID)
		}
	}
}

func capRows(rows []client.Row, limit int) []client.Row {
	if limit <= 0 {
		return rows
	}
	for i := range rows {
		rows[i].Titles = capTitles(rows[i].Titles, limit)
	}
	return rows
}

func capTitles(titles []client.Title, limit int) []client.Title {
	if limit > 0 && len(titles) > limit {
		return titles[:limit]
	}
	return titles
}
