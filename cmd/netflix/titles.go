package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdTitle shows one title's detail, the same record the web app's title page
// loads.
func cmdTitle(args []string) error {
	fs, cf := newCommonFlags("title")
	similar := fs.Bool("similar", false, "resolve the similar-title ids into names")
	parseFlags(fs, args)
	id, err := titleOperand(fs, "netflix title <id|url>")
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	detail, err := cl.Catalog.Detail(id)
	if err != nil {
		return err
	}
	if !*similar || len(detail.Similar) == 0 {
		return output(cf, detail, func() { printDetail(detail) })
	}
	similars, err := cl.Catalog.Details(detail.Similar)
	if err != nil {
		return err
	}
	return output(cf, map[string]any{"title": detail, "similar": similars}, func() {
		printDetail(detail)
		fmt.Println("\nsimilar:")
		for _, s := range similars {
			fmt.Printf("  • %s  [%d]\n", s.Title, s.ID)
		}
	})
}

func printTitles(titles []client.Title) {
	for _, t := range titles {
		fmt.Printf("• %s  [%d]\n", t.Title, t.ID)
		if kind := strings.ToLower(t.Kind); kind != "" {
			fmt.Printf("  %s\n", kind)
		}
		fmt.Printf("  %s\n", t.URL)
	}
}

// printDetail renders a title the way a reader wants it: the human-readable
// certification rather than Netflix's internal maturity number.
func printDetail(d client.TitleDetail) {
	fmt.Printf("%s  [%d]\n", d.Title, d.ID)
	fmt.Printf("  %s\n", strings.Join(detailMeta(d), " · "))
	if len(d.Genres) > 0 {
		fmt.Printf("  %s\n", strings.Join(d.Genres, ", "))
	}
	if d.Synopsis != "" {
		fmt.Printf("\n%s\n\n", d.Synopsis)
	}
	printPeople("cast", d.Cast)
	printPeople("director", d.Directors)
	printPeople("writer", d.Writers)
	if status := detailStatus(d); status != "" {
		fmt.Printf("  %s\n", status)
	}
	fmt.Printf("  %s\n", d.URL)
}

func detailMeta(d client.TitleDetail) []string {
	meta := []string{strings.ToLower(d.Kind)}
	if d.Year > 0 {
		meta = append(meta, fmt.Sprint(d.Year))
	}
	if runtime := d.Runtime(); runtime != "" {
		meta = append(meta, runtime)
	}
	if d.Rating != "" {
		meta = append(meta, d.Rating)
	}
	return meta
}

func detailStatus(d client.TitleDetail) string {
	status := strings.ToLower(strings.ReplaceAll(d.WatchStatus, "_", " "))
	if d.InMyList {
		if status != "" {
			status += " · "
		}
		status += "in my list"
	}
	return status
}

func printPeople(label string, names []string) {
	if len(names) == 0 {
		return
	}
	fmt.Printf("  %s: %s\n", label, strings.Join(names, ", "))
}
