package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
	"github.com/seifreed/netflixcli/internal/session"
)

// cmdProfiles lists the account's profiles and marks the active one.
func cmdProfiles(args []string) error {
	fs, cf := newCommonFlags("profiles")
	parseFlags(fs, args)
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	profiles, err := cl.Profiles()
	if err != nil {
		return err
	}
	return output(cf, profiles, func() {
		for _, p := range profiles {
			fmt.Println(profileLine(p))
		}
	})
}

func profileLine(p client.Profile) string {
	marker := " "
	if p.Current {
		marker = "*"
	}
	line := fmt.Sprintf("%s %-20s %s", marker, p.Name, p.GUID)
	var tags []string
	if p.IsKids {
		tags = append(tags, "kids")
	}
	if p.IsPinLocked {
		tags = append(tags, "pin-locked")
	}
	if len(tags) > 0 {
		line += "  (" + strings.Join(tags, ", ") + ")"
	}
	return line
}

// cmdProfileUse re-points the stored session at another profile, the way the
// web app's profile switcher does.
func cmdProfileUse(args []string) error {
	fs, cf := newCommonFlags("profile use")
	parseFlags(fs, args)
	want, err := operand(fs, "netflix profile use <name|guid>")
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	profile, updated, err := cl.UseProfile(want)
	if err != nil {
		return err
	}
	if err := session.SaveSession(session.Session{Cookie: updated}); err != nil {
		return err
	}
	return output(cf, profile, func() {
		fmt.Fprintf(os.Stderr, "session now acts as %q\n", profile.Name)
	})
}

// cmdProfile dispatches the profile subcommands.
func cmdProfile(args []string) error {
	if len(args) > 0 && args[0] == "use" {
		return cmdProfileUse(args[1:])
	}
	if len(args) > 0 && args[0] == "list" {
		return cmdProfiles(args[1:])
	}
	return cmdProfiles(args)
}

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
	viewings, err := cl.History("")
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
