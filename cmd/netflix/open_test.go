package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubLaunch captures the URL `open` would hand to the browser, instead of
// opening one.
func stubLaunch(t *testing.T) *string {
	t.Helper()
	var opened string
	original := launch
	launch = func(target string) error {
		opened = target
		return nil
	}
	t.Cleanup(func() { launch = original })
	return &opened
}

// Handing a URL to the browser needs no session. `open` used to build a client
// first, which bootstraps the session and switches profile when one is
// configured, so it failed on a stale cookie it was never going to use.
func TestOpenNeedsNoSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NETFLIX_CONFIG_DIR", dir) // no session.json, and a default profile
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[defaults]\nprofile = \"Kid\"\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	t.Setenv("NETFLIX_BASE_URL", "https://www.netflix.com")
	opened := stubLaunch(t)

	code, out := runCommand(t, "open", "80100172")
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d without a session", code, exitOK)
	}
	want := "https://www.netflix.com/title/80100172"
	if *opened != want {
		t.Errorf("opened %q, want %q", *opened, want)
	}
	if !strings.Contains(out, want) {
		t.Errorf("output %q does not report the url", out)
	}
}

func TestOpenWatchGoesToThePlayer(t *testing.T) {
	t.Setenv("NETFLIX_CONFIG_DIR", t.TempDir())
	t.Setenv("NETFLIX_BASE_URL", "https://www.netflix.com/")
	opened := stubLaunch(t)

	if code, _ := runCommand(t, "open", "--watch", "https://www.netflix.com/title/70095139"); code != exitOK {
		t.Fatalf("exit code = %d", code)
	}
	if want := "https://www.netflix.com/watch/70095139"; *opened != want {
		t.Errorf("opened %q, want %q — a trailing slash on the host must not double up", *opened, want)
	}
}
