package config

import (
	"os"
	"path/filepath"
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
