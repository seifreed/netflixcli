package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirHonoursOverride(t *testing.T) {
	t.Setenv("NETFLIX_CONFIG_DIR", "/tmp/netflix-test")
	if got := Dir(); got != "/tmp/netflix-test" {
		t.Errorf("Dir = %q, want the override", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NETFLIX_CONFIG_DIR", dir)
	type state struct {
		Cookie string `json:"cookie"`
	}
	if err := Save("session.json", state{Cookie: "NetflixId=x"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var got state
	if err := Load("session.json", &got); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Cookie != "NetflixId=x" {
		t.Errorf("cookie = %q, want it to survive the round trip", got.Cookie)
	}
	// The file holds a session cookie, so it must not be world-readable.
	info, err := os.Stat(filepath.Join(dir, "session.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("mode = %v, want no group or other access", perm)
	}
}

// A state name is always a fixed literal in this CLI; anything path-like is a
// bug, and must not be able to write outside the config dir.
func TestSaveRejectsPathLikeNames(t *testing.T) {
	t.Setenv("NETFLIX_CONFIG_DIR", t.TempDir())
	for _, name := range []string{"", ".", "..", "../escape.json", "sub/file.json"} {
		if err := Save(name, struct{}{}); err == nil {
			t.Errorf("Save(%q) succeeded, want a refusal", name)
		}
		if err := Load(name, &struct{}{}); err == nil {
			t.Errorf("Load(%q) succeeded, want a refusal", name)
		}
	}
}

func TestLoadConfigTreatsMissingFileAsEmpty(t *testing.T) {
	t.Setenv("NETFLIX_CONFIG_DIR", t.TempDir())
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Auth.Cookie != "" || cfg.Defaults.Lang != "" {
		t.Errorf("config = %+v, want the zero value", cfg)
	}
}

func TestLoadConfigReadsTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NETFLIX_CONFIG_DIR", dir)
	toml := "[auth]\ncookie = \"NetflixId=x\"\n\n[defaults]\nprofile = \"Ada\"\nlang = \"en\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(toml), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Auth.Cookie != "NetflixId=x" || cfg.Defaults.Profile != "Ada" || cfg.Defaults.Lang != "en" {
		t.Errorf("config = %+v, want the file's values", cfg)
	}
}

// The files are 0600, but a directory other users can write lets them replace
// the stored session with one of their own — after which the CLI reads and
// writes somebody else's account. MkdirAll only sets the mode when it creates
// the directory, so one that was already there keeps whatever it had.
func TestSharedDirWarning(t *testing.T) {
	for perm, want := range map[os.FileMode]bool{
		0o700: false,
		0o500: false,
		0o750: true, // readable by the group
		0o770: true,
		0o777: true,
		0o701: true, // others can traverse and write
	} {
		dir := t.TempDir()
		if err := os.Chmod(dir, perm); err != nil {
			t.Fatalf("chmod %#o: %v", perm, err)
		}
		t.Setenv("NETFLIX_CONFIG_DIR", dir)
		warning := SharedDirWarning()
		if got := warning != ""; got != want {
			t.Errorf("mode %#o warned = %v, want %v (%q)", perm, got, want, warning)
		}
		if want && !strings.Contains(warning, "chmod 700") {
			t.Errorf("mode %#o: warning %q does not say how to fix it", perm, warning)
		}
		os.Chmod(dir, 0o700) // let t.TempDir clean up
	}
}

// A directory that is not there yet is not a warning; Save creates it 0700.
func TestSharedDirWarningIgnoresAMissingDir(t *testing.T) {
	t.Setenv("NETFLIX_CONFIG_DIR", filepath.Join(t.TempDir(), "not-created-yet"))
	if warning := SharedDirWarning(); warning != "" {
		t.Errorf("warned about a directory that does not exist: %q", warning)
	}
}
