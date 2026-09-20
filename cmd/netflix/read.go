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
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return fmt.Errorf("usage: netflix search <query>")
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
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

// cmdTitle shows one title's detail, the same record the web app's title page
// loads.
func cmdTitle(args []string) error {
	fs, cf := newCommonFlags("title")
	similar := fs.Bool("similar", false, "resolve the similar-title ids into names")
	parseFlags(fs, args)
	raw := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if raw == "" {
		return fmt.Errorf("usage: netflix title <id|url>")
	}
	id, err := client.ParseTitleID(raw)
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	detail, err := cl.Detail(id)
	if err != nil {
		return err
	}
	if *similar && len(detail.Similar) > 0 {
		similars, err := cl.Details(detail.Similar)
		if err != nil {
			return err
		}
		if done, err := emitStructured(cf, map[string]any{"title": detail, "similar": similars}); done {
			return err
		}
		printDetail(detail)
		fmt.Println("\nsimilar:")
		for _, s := range similars {
			fmt.Printf("  • %s  [%d]\n", s.Title, s.ID)
		}
		return nil
	}
	if done, err := emitStructured(cf, detail); done {
		return err
	}
	printDetail(detail)
	return nil
}

func printDetail(d client.TitleDetail) {
	fmt.Printf("%s  [%d]\n", d.Title, d.ID)
	meta := []string{strings.ToLower(d.Kind)}
	if d.Year > 0 {
		meta = append(meta, fmt.Sprint(d.Year))
	}
	if r := d.Runtime(); r != "" {
		meta = append(meta, r)
	}
	if d.Rating != "" {
		meta = append(meta, d.Rating)
	}
	fmt.Printf("  %s\n", strings.Join(meta, " · "))
	if len(d.Genres) > 0 {
		fmt.Printf("  %s\n", strings.Join(d.Genres, ", "))
	}
	if d.Synopsis != "" {
		fmt.Printf("\n%s\n\n", d.Synopsis)
	}
	printPeople("cast", d.Cast)
	printPeople("director", d.Directors)
	printPeople("writer", d.Writers)
	status := d.WatchStatus
	if d.InMyList {
		status += " · in my list"
	}
	if status != "" {
		fmt.Printf("  %s\n", strings.ToLower(strings.ReplaceAll(status, "_", " ")))
	}
	fmt.Printf("  %s\n", d.URL)
}

func printPeople(label string, names []string) {
	if len(names) == 0 {
		return
	}
	fmt.Printf("  %s: %s\n", label, strings.Join(names, ", "))
}

// cmdBrowse prints the rows of a browse surface — the home page by default.
func cmdBrowse(args []string) error {
	fs, cf := newCommonFlags("browse")
	surface := fs.String("surface", "", "home, my-netflix, latest, games, or a genre id")
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
	if done, err := emitStructured(cf, rows); done {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("(no rows — the surface may not exist in this region)")
		return nil
	}
	for _, row := range rows {
		fmt.Printf("\n%s\n", row.Name)
		for _, t := range row.Titles {
			fmt.Printf("  • %s  [%d]\n", t.Title, t.ID)
		}
	}
	return nil
}

func capRows(rows []client.Row, limit int) []client.Row {
	if limit <= 0 {
		return rows
	}
	for i := range rows {
		if len(rows[i].Titles) > limit {
			rows[i].Titles = rows[i].Titles[:limit]
		}
	}
	return rows
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
		titles := row.Titles
		if *limit > 0 && len(titles) > *limit {
			titles = titles[:*limit]
		}
		if done, err := emitStructured(cf, titles); done {
			return err
		}
		if len(titles) == 0 {
			fmt.Printf("(%s is empty for this profile)\n", row.Name)
			return nil
		}
		printTitles(titles)
		return nil
	}
}
