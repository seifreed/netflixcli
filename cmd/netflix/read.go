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
	cl := newClient(cf)
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
