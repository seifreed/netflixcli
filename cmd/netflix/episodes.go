package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdSeasons lists a show's seasons.
func cmdSeasons(args []string) error {
	fs, cf := newCommonFlags("seasons")
	parseFlags(fs, args)
	id, err := showArg(fs.Args(), "seasons")
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	seasons, err := cl.Seasons(id)
	if err != nil {
		return err
	}
	if done, err := emitStructured(cf, seasons); done {
		return err
	}
	for _, s := range seasons {
		fmt.Printf("%2d. %-28s %d episodes  [%d]\n", s.Number, s.Title, s.Episodes, s.ID)
	}
	return nil
}

// cmdEpisodes lists one season's episodes — the first season unless --season
// says otherwise, or every season with --all.
func cmdEpisodes(args []string) error {
	fs, cf := newCommonFlags("episodes")
	season := fs.Int("season", 0, "season number (default: the first)")
	all := fs.Bool("all", false, "list every season")
	limit := fs.Int("limit", 0, "max episodes per season")
	parseFlags(fs, args)
	id, err := showArg(fs.Args(), "episodes")
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	seasons, err := cl.Seasons(id)
	if err != nil {
		return err
	}
	wanted := seasons
	if !*all {
		picked := seasons[0]
		if *season > 0 {
			if picked, err = client.SeasonByNumber(seasons, *season); err != nil {
				return err
			}
		}
		wanted = []client.Season{picked}
	}

	type seasonEpisodes struct {
		Season   client.Season    `json:"season"`
		Episodes []client.Episode `json:"episodes"`
	}
	out := make([]seasonEpisodes, 0, len(wanted))
	for _, s := range wanted {
		episodes, err := cl.Episodes(s.ID, *limit)
		if err != nil {
			return err
		}
		out = append(out, seasonEpisodes{Season: s, Episodes: episodes})
	}
	if done, err := emitStructured(cf, out); done {
		return err
	}
	for _, block := range out {
		fmt.Printf("\n%s\n", block.Season.Title)
		for _, e := range block.Episodes {
			line := fmt.Sprintf("%3d. %s", e.Number, e.Title)
			if r := e.Runtime(); r != "" {
				line += "  (" + r + ")"
			}
			if e.ProgressSec > 0 {
				line += "  ▸ resumes"
			}
			fmt.Println(line)
			if e.Synopsis != "" {
				fmt.Printf("     %s\n", e.Synopsis)
			}
		}
	}
	return nil
}

func showArg(args []string, command string) (int, error) {
	raw := strings.TrimSpace(strings.Join(args, " "))
	if raw == "" {
		return 0, fmt.Errorf("usage: netflix %s <show-id|url>", command)
	}
	return client.ParseTitleID(raw)
}
