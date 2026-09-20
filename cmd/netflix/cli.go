package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	toon "github.com/toon-format/toon-go"

	"github.com/seifreed/netflixcli/internal/browser"
	"github.com/seifreed/netflixcli/internal/client"
	"github.com/seifreed/netflixcli/internal/config"
	"github.com/seifreed/netflixcli/internal/session"
)

// stderrLogf routes diagnostics to stderr, prefixed, so they never mix with
// --json data on stdout.
var stderrLogf = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "netflix: "+format+"\n", args...)
}

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
func newClient(c *common) (*client.Client, error) {
	cl := client.New()
	cl.Logf = stderrLogf
	if u := os.Getenv("NETFLIX_BASE_URL"); u != "" {
		cl.BaseURL = u
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		stderrLogf("config.toml could not be read (%v) — using defaults", err)
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
		fetcher, fetchErr := browser.New(endpoint)
		if fetchErr != nil {
			cl.SetFetcher(func(string) (string, error) { return "", fetchErr })
		} else {
			cl.SetFetcher(func(rawURL string) (string, error) { return fetcher.Fetch(rawURL, 60*time.Second) })
		}
	}
	if profile := firstNonEmpty(c.profile, cfg.Defaults.Profile); profile != "" {
		if _, _, err := cl.UseProfile(profile); err != nil {
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

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func emitJSONL(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var rows []json.RawMessage
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &rows); err != nil {
			return err
		}
	} else {
		rows = []json.RawMessage{raw}
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(os.Stdout, string(row)); err != nil {
			return err
		}
	}
	return nil
}

// toonEncode renders v as TOON, routing through JSON first so field names match
// --json exactly.
func toonEncode(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var generic any
	_ = json.Unmarshal(raw, &generic)
	return toon.MarshalString(generic)
}

func emitTOON(v any) error {
	s, err := toonEncode(v)
	if err != nil || s == "" {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, s)
	return err
}

// emitStructured emits v as --toon / --jsonl / --json and reports whether it did,
// so callers fall through to the human view only when no such flag is set.
func emitStructured(cf *common, v any) (emitted bool, err error) {
	switch {
	case cf.toon:
		return true, emitTOON(v)
	case cf.jsonl:
		return true, emitJSONL(v)
	case cf.jsonOut:
		return true, emitJSON(v)
	}
	return false, nil
}
