package main

import (
	"fmt"
	"os"
	"strings"

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

func emitViewingsCSV(viewings []client.Viewing) error {
	if _, err := fmt.Fprintln(os.Stdout, "date,title"); err != nil {
		return err
	}
	for _, v := range viewings {
		if _, err := fmt.Fprintf(os.Stdout, "%s,%s\n", v.Date, csvField(v.Title)); err != nil {
			return err
		}
	}
	return nil
}

// csvField quotes a field that would otherwise break the row.
func csvField(s string) string {
	if !strings.ContainsAny(s, `,"`+"\n") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
