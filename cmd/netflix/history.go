package main

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdHistory prints the viewing activity of the profile the session is acting
// as — another one with --profile.
func cmdHistory(args []string) error {
	fs, cf := newCommonFlags("history")
	limit := fs.Int("limit", 0, "max entries to return (0 = all)")
	asCSV := fs.Bool("csv", false, "emit CSV instead of a table")
	parseFlags(fs, args)
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	viewings, err := cl.Account.History("")
	if err != nil {
		return err
	}
	if *limit > 0 && len(viewings) > *limit {
		viewings = viewings[:*limit]
	}
	if *asCSV {
		return emitViewingsCSV(viewings)
	}
	return output(cf, viewings, func() {
		if len(viewings) == 0 {
			fmt.Println("(no viewing activity for this profile)")
			return
		}
		for _, v := range viewings {
			fmt.Printf("%s  %s\n", v.Date, v.Title)
		}
	})
}

// emitViewingsCSV writes the activity as CSV. The quoting is encoding/csv's, the
// same package internal/client reads Netflix's own export with.
func emitViewingsCSV(viewings []client.Viewing) error {
	w := csv.NewWriter(os.Stdout)
	rows := [][]string{{"date", "title"}}
	for _, v := range viewings {
		rows = append(rows, []string{v.Date, v.Title})
	}
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	return w.Error()
}
