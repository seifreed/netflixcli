package main

import (
	"fmt"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdBrowse prints the rows of a browse surface — the home page by default.
func cmdBrowse(args []string) error {
	fs, cf := newCommonFlags("browse")
	surface := fs.String("surface", "", "home, my-netflix, or a genre id")
	limit := fs.Int("limit", 0, "max titles per row (0 = whatever the page carries)")
	all := fs.Bool("all", false, "every row, not just the eight the page renders")
	parseFlags(fs, args)
	if positional := optionalOperand(fs); positional != "" {
		*surface = positional
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	fetch := cl.Library.Browse
	if *all {
		fetch = cl.Library.AllRows
	}
	rows, err := fetch(*surface)
	if err != nil {
		return err
	}
	rows = capRows(rows, *limit)
	return output(cf, rows, func() {
		if len(rows) == 0 {
			fmt.Println("(no rows — the surface may not exist in this region)")
			return
		}
		printRows(rows, bullet)
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
		row, err := cl.Library.Feed(feed)
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

// printRows prints each row under its name. mark renders what precedes a title:
// a bullet, or its rank where the order is the point.
func printRows(rows []client.Row, mark func(i int) string) {
	for _, row := range rows {
		fmt.Printf("\n%s\n", row.Name)
		if len(row.Titles) == 0 {
			fmt.Println("  (empty)")
		}
		for i, t := range row.Titles {
			fmt.Printf("  %s %s  [%d]\n", mark(i), t.Title, t.ID)
		}
	}
}

func bullet(int) string { return "•" }

func rank(i int) string { return fmt.Sprintf("%2d.", i+1) }

// capRows caps every row's titles. Like capTitles it leaves the caller's rows
// alone, so a capped view cannot be mistaken for the whole one.
func capRows(rows []client.Row, limit int) []client.Row {
	if limit <= 0 {
		return rows
	}
	capped := make([]client.Row, len(rows))
	for i, row := range rows {
		row.Titles = capTitles(row.Titles, limit)
		capped[i] = row
	}
	return capped
}

func capTitles(titles []client.Title, limit int) []client.Title {
	if limit > 0 && len(titles) > limit {
		return titles[:limit]
	}
	return titles
}

// cmdTop prints Netflix's top 10 rows. They are not in the page: they sit past
// the twentieth row, so this pages through the rows to reach them.
func cmdTop(args []string) error {
	fs, cf := newCommonFlags("top")
	limit := fs.Int("limit", 0, "max titles per list")
	parseFlags(fs, args)
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	rows, err := cl.Library.Top()
	if err != nil {
		return err
	}
	rows = capRows(rows, *limit)
	return output(cf, rows, func() { printRows(rows, rank) })
}
