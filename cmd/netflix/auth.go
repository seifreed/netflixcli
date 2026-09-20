package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
	"github.com/seifreed/netflixcli/internal/config"
	"github.com/seifreed/netflixcli/internal/cookie"
	"github.com/seifreed/netflixcli/internal/session"
)

// cmdLogin lifts the Netflix session straight out of a browser's cookie store.
// The user signs in to netflix.com normally and the CLI reads the resulting
// cookies — the CLI never sees or asks for a password.
func cmdLogin(args []string) error {
	fs, cf := newOutputFlags("login")
	from := fs.String("from-browser", "", "browser to read cookies from: chrome|chromium|firefox|safari|edge|brave (empty = any)")
	parseFlags(fs, args)
	s, err := session.CookiesFromBrowser(*from)
	if err != nil {
		return err
	}
	if err := session.SaveSession(s); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "saved session cookie (%d bytes) → %s/session.json\n", len(s.Cookie), config.Dir())
	if !cookie.LooksAuthenticated(s.Cookie) {
		stderrLogf("cookie carries no NetflixId — sign in to www.netflix.com in that browser, then re-run")
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	user, probeErr := cl.Account.Whoami()
	if err := output(cf, loginResult(s, user, probeErr), func() { printUser(user) }); err != nil {
		return err
	}
	return probeErr
}

func loginResult(s session.Session, user client.UserInfo, probeErr error) map[string]any {
	return map[string]any{
		"saved":        true,
		"hasCookie":    s.Cookie != "",
		"hasNetflixId": cookie.LooksAuthenticated(s.Cookie),
		"accepted":     probeErr == nil,
		"user":         user,
	}
}

// cmdImportHar imports a session from a DevTools HAR export ("Save all as HAR
// with sensitive data").
func cmdImportHar(args []string) error {
	fs, cf := newOutputFlags("import-har")
	file := fs.String("file", "", "path to the .har export ('-' for stdin)")
	parseFlags(fs, args)
	if *file == "" {
		return usagef("usage: netflix import-har --file netflix.har")
	}
	data, err := readHAROrStdin(*file)
	if err != nil {
		return err
	}
	s, err := session.ParseHAR(data)
	if err != nil {
		return err
	}
	if err := session.SaveSession(s); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "imported session cookie (%d bytes) → %s/session.json\n", len(s.Cookie), config.Dir())
	return emitSavedSession(cf, s)
}

// cmdSetCookie seeds a raw Cookie header (copied from DevTools) into the cache.
func cmdSetCookie(args []string) error {
	fs, cf := newOutputFlags("set-cookie")
	stdin := fs.Bool("stdin", false, "read the cookie from stdin")
	parseFlags(fs, args)
	var rawCookie string
	if *stdin {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, cookie.MaxHeaderBytes+1))
		if err != nil {
			return err
		}
		if len(b) > cookie.MaxHeaderBytes {
			return fmt.Errorf("cookie header is too large (maximum %d KiB)", cookie.MaxHeaderBytes>>10)
		}
		rawCookie = strings.TrimSpace(string(b))
	} else if fs.NArg() > 0 {
		rawCookie = strings.Join(fs.Args(), " ")
	}
	if rawCookie == "" {
		return usagef("usage: netflix set-cookie '<cookie header>'  (or --stdin)")
	}
	s := session.LoadSession()
	s.Cookie = rawCookie
	if err := session.SaveSession(s); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "saved cookie → %s/session.json\n", config.Dir())
	if !cookie.LooksAuthenticated(rawCookie) {
		stderrLogf("cookie has no NetflixId entry — run `netflix whoami` to check whether it is accepted")
	}
	return emitSavedSession(cf, s)
}

func emitSavedSession(cf *common, s session.Session) error {
	return output(cf, map[string]any{
		"saved":        true,
		"hasCookie":    s.Cookie != "",
		"hasNetflixId": cookie.LooksAuthenticated(s.Cookie),
	}, func() {})
}

// cmdWhoami reports the account the current session belongs to.
func cmdWhoami(args []string) error {
	fs, cf := newCommonFlags("whoami")
	parseFlags(fs, args)
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	user, whoamiErr := cl.Account.Whoami()
	profile, _ := cl.Account.CurrentProfile() // free once Whoami has bootstrapped
	report := map[string]any{
		"hasCookie": cl.Cookie != "",
		"accepted":  whoamiErr == nil,
		"user":      user,
		"profile":   profile,
	}
	if err := output(cf, report, func() { printSession(user, profile) }); err != nil {
		return err
	}
	return whoamiErr
}

func printSession(user client.UserInfo, profile client.Profile) {
	printUser(user)
	if profile.Name != "" {
		fmt.Printf("  acting as: %s (%s)\n", profile.Name, profile.GUID)
	}
}

func printUser(u client.UserInfo) {
	if u.Name == "" {
		// Netflix named no account, which means the session was refused. The
		// error on stderr says so; printing " ()" to stdout says nothing.
		return
	}
	fmt.Printf("%s (%s)\n", u.Name, u.MembershipStatus)
	if u.AccountOwnerName != "" && u.AccountOwnerName != u.Name {
		fmt.Printf("  account: %s\n", u.AccountOwnerName)
	}
	if u.CountryOfSignup != "" {
		fmt.Printf("  country: %s\n", u.CountryOfSignup)
	}
	if u.MemberSince != "" {
		fmt.Printf("  member since: %s\n", u.MemberSince)
	}
	if u.NumProfiles > 0 {
		fmt.Printf("  profiles: %d\n", u.NumProfiles)
	}
}

func readHAROrStdin(path string) ([]byte, error) {
	var reader io.Reader
	if path == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, session.MaxHARBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > session.MaxHARBytes {
		return nil, fmt.Errorf("HAR is too large (maximum %d MiB)", session.MaxHARBytes>>20)
	}
	return data, nil
}
