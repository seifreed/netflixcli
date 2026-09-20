package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdSearch queries the catalogue the way the web app's search page does.
func cmdSearch(args []string) error {
	fs, cf := newCommonFlags("search")
	limit := fs.Int("limit", 0, "max titles to return (0 = one page, 48)")
	parseFlags(fs, args)
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return fmt.Errorf("usage: netflix search <query>")
	}
	cl := newClient(cf)
	titles, err := cl.Search(query, *limit)
	if err != nil {
		return err
	}
	if done, err := emitStructured(cf, titles); done {
		return err
	}
	if len(titles) == 0 {
		fmt.Printf("(no results for %q)\n", query)
		return nil
	}
	printTitles(titles)
	return nil
}

func printTitles(titles []client.Title) {
	for _, t := range titles {
		fmt.Printf("• %s  [%d]\n", t.Title, t.ID)
		kind := strings.ToLower(t.Kind)
		if kind != "" {
			fmt.Printf("  %s\n", kind)
		}
		fmt.Printf("  %s\n", t.URL)
	}
}
