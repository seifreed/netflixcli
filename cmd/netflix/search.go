package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdSearch queries the catalogue the way the web app's search page does.
func cmdSearch(args []string) error {
	fs, cf := newCommonFlags("search")
	limit := fs.Int("limit", 0, "how many titles to return; pages past the first 48 (0 = one page)")
	parseFlags(fs, args)
	query, err := operand(fs, "netflix search <query>")
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	titles, err := cl.Catalog.Search(query, *limit)
	if err != nil {
		return err
	}
	return output(cf, titles, func() {
		if len(titles) == 0 {
			fmt.Printf("(no results for %q)\n", query)
			return
		}
		printTitles(titles)
	})
}

// cmdGenres lists the genres this region offers, optionally filtered. The ids
// are what `browse` needs and Netflix never shows them in the UI.
func cmdGenres(args []string) error {
	fs, cf := newCommonFlags("genres")
	parseFlags(fs, args)
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	genres, err := cl.Catalog.Genres()
	if err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	genres = client.MatchGenres(genres, query)
	return output(cf, genres, func() {
		if len(genres) == 0 {
			fmt.Printf("(no genre matches %q)\n", query)
			return
		}
		for _, g := range genres {
			fmt.Printf("%-40s %d\n", g.Title, g.Number)
		}
	})
}
