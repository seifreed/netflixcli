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
	if err := noOperands(fs, "profiles"); err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	profiles, err := cl.Account.Profiles()
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
	profile, updated, err := cl.Account.UseProfile(want)
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
