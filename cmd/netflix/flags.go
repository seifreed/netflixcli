package main

// Flag plumbing shared by every command: the flags they all accept, how the
// client is built from them, and how positional arguments are read.

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/seifreed/netflixcli/internal/browser"
	"github.com/seifreed/netflixcli/internal/client"
	"github.com/seifreed/netflixcli/internal/config"
	"github.com/seifreed/netflixcli/internal/session"
)

// browserFetchTimeout is how long a page may take through the user's Chrome. It
// is generous because an interstitial may be waiting for them to clear it.
const browserFetchTimeout = 60 * time.Second

// common flags shared by every subcommand.
type common struct {
	lang            string
	profile         string
	jsonOut         bool
	jsonl           bool
	toon            bool
	browser         bool
	browserEndpoint string
}

func newCommonFlags(name string) (*flag.FlagSet, *common) {
	fs, c := newOutputFlags(name)
	fs.StringVar(&c.lang, "lang", "", "UI language for titles and labels, e.g. es-ES or en")
	fs.StringVar(&c.profile, "profile", "", "profile guid or name to act as")
	return fs, c
}

func newOutputFlags(name string) (*flag.FlagSet, *common) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	c := &common{}
	fs.BoolVar(&c.jsonOut, "json", false, "emit raw JSON to stdout")
	fs.BoolVar(&c.jsonl, "jsonl", false, "emit one JSON object per line to stdout")
	fs.BoolVar(&c.toon, "toon", false, "emit TOON (token-oriented, fewer tokens than JSON) to stdout")
	fs.BoolVar(&c.browser, "browser", false, "fetch through the existing Chrome via CDP; never launches Chrome")
	fs.StringVar(&c.browserEndpoint, "browser-endpoint", "", "local Chrome CDP endpoint (default: NETFLIX_CHROME_CDP_URL or 127.0.0.1:9222)")
	return fs, c
}

// newClient builds a client from flags, config and the cached browser session.
// With --profile (or a configured default) it acts as that profile for this
// invocation only: the switch happens in memory and the stored session keeps
// pointing where it did. `profile use` is what changes it for good.
// baseURL is the page host the CLI talks to: the public site, unless
// NETFLIX_BASE_URL points at a debugging proxy or a mock.
//
// It must be an http(s) origin. Whatever is built from it is handed to the
// system browser by `open`, and to the operating system a "URL" that is not one
// is just an argument — `file://`, a custom scheme, or a value beginning with a
// dash that the opener reads as a flag.
func baseURL() (string, error) {
	raw := os.Getenv("NETFLIX_BASE_URL")
	if raw == "" {
		return client.BaseURL, nil
	}
	if !isWebURL(raw) {
		return "", usagef("NETFLIX_BASE_URL must be an http(s) URL, got %q", raw)
	}
	return raw, nil
}

// isWebURL reports whether raw is an absolute http(s) URL with a host.
func isWebURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func newClient(c *common) (*client.Client, error) {
	base, err := baseURL()
	if err != nil {
		return nil, err
	}
	cl := client.New()
	cl.Logf = stderrLogf
	cl.BaseURL = base
	if u := os.Getenv("NETFLIX_GRAPHQL_URL"); u != "" {
		cl.GraphQLURL = u
	}
	cfg, cfgErr := config.LoadConfig()
	if cfgErr != nil {
		stderrLogf("config.toml could not be read (%v) — using defaults", cfgErr)
	}
	if v := firstNonEmpty(c.lang, cfg.Defaults.Lang); v != "" {
		cl.Lang = v
	}
	if cached := session.LoadSession(); cached.Cookie != "" {
		cl.Cookie = cached.Cookie
	} else if cfg.Auth.Cookie != "" {
		cl.Cookie = cfg.Auth.Cookie
	}
	if c.browser || c.browserEndpoint != "" || os.Getenv("NETFLIX_CHROME_CDP_URL") != "" {
		endpoint := firstNonEmpty(c.browserEndpoint, os.Getenv("NETFLIX_CHROME_CDP_URL"), browser.DefaultEndpoint)
		fetcher, err := browser.New(endpoint)
		if err != nil {
			return nil, err
		}
		cl.SetFetcher(func(rawURL string) (string, error) { return fetcher.Fetch(rawURL, browserFetchTimeout) })
	}
	if profile := firstNonEmpty(c.profile, cfg.Defaults.Profile); profile != "" {
		if _, _, err := cl.Account.UseProfile(profile); err != nil {
			return nil, err
		}
	}
	return cl, nil
}

func parseFlags(fs *flag.FlagSet, args []string) {
	_ = fs.Parse(reorderArgs(fs, args))
}

// reorderArgs lets flags appear anywhere among positional args (the stdlib flag
// parser stops at the first positional). Honours bool flags, --flag=value, and --.
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' && (a[1] < '0' || a[1] > '9') && a[1] != '.' {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if strings.IndexByte(name, '=') >= 0 {
				continue
			}
			if f := fs.Lookup(name); f != nil && !isBoolFlag(f) && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	out := make([]string, 0, len(flags)+1+len(positional))
	out = append(out, flags...)
	out = append(out, "--")
	return append(out, positional...)
}

func isBoolFlag(f *flag.Flag) bool {
	bf, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && bf.IsBoolFlag()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// usageError is a command rejecting how it was called, before doing any work.
// main exits 2 for it — "the invocation was wrong" — rather than 1, which means
// the command ran and failed.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func usagef(format string, args ...any) error {
	return usageError{fmt.Errorf(format, args...)}
}

// noOperands rejects arguments a command does not take. Without it a mistyped
// subcommand ran the parent instead and looked like it had worked: `mylist ad
// 123` listed My List and exited 0, having added nothing.
func noOperands(fs *flag.FlagSet, name string) error {
	extra := fs.Args()
	if len(extra) == 0 {
		return nil
	}
	if subs := subcommandsOf(name); len(subs) > 0 {
		return usagef("netflix %s takes no arguments — did you mean `netflix %s %s <id|url>`?",
			name, name, strings.Join(subs, "|"))
	}
	return usagef("netflix %s takes no arguments, got %q", name, strings.Join(extra, " "))
}

// optionalOperand returns the command's positional argument, joined and
// trimmed, or "" when it was not given.
func optionalOperand(fs *flag.FlagSet) string {
	return strings.TrimSpace(strings.Join(fs.Args(), " "))
}

// operand returns the command's positional argument, or a usage error naming
// the right spelling. Operands are validated before the client is built, so a
// typo costs no request.
func operand(fs *flag.FlagSet, usage string) (string, error) {
	value := optionalOperand(fs)
	if value == "" {
		return "", usagef("usage: %s", usage)
	}
	return value, nil
}

// titleOperand returns the positional argument parsed as a Netflix title id; it
// accepts a bare id or any netflix.com URL carrying one.
func titleOperand(fs *flag.FlagSet, usage string) (int, error) {
	raw, err := operand(fs, usage)
	if err != nil {
		return 0, err
	}
	id, err := client.ParseTitleID(raw)
	if err != nil {
		return 0, usageError{err} // an operand the CLI cannot read is a wrong call
	}
	return id, nil
}
