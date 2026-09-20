// Package config handles on-disk state under ~/.netflix: the user-authored
// config.toml (session + defaults) and the machine-managed session cache.
// NETFLIX_CONFIG_DIR overrides the directory.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const configFile = "config.toml"

// Dir is ~/.netflix (or $NETFLIX_CONFIG_DIR).
func Dir() string {
	if d := os.Getenv("NETFLIX_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".netflix"
	}
	return filepath.Join(home, ".netflix")
}

// Config mirrors ~/.netflix/config.toml.
type Config struct {
	Auth struct {
		Cookie string `toml:"cookie"` // browser session (NetflixId / SecureNetflixId)
	} `toml:"auth"`
	Defaults struct {
		Profile string `toml:"profile"` // profile guid used by default
		Lang    string `toml:"lang"`    // UI language, e.g. "es-ES" or "en"
	} `toml:"defaults"`
}

// LoadConfig reads config.toml. A missing file is not an error (empty config).
func LoadConfig() (Config, error) {
	var c Config
	p := filepath.Join(Dir(), configFile)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, fmt.Errorf("stat config %s: %w", p, err)
	}
	_, err := toml.DecodeFile(p, &c)
	return c, err
}

// ensureDir creates the config dir (0700; it holds secrets) if absent. It does
// not relax or tighten one that already exists — see SharedDirWarning.
func ensureDir() error { return os.MkdirAll(Dir(), 0o700) }

// SharedDirWarning describes how the config dir is exposed to other users on
// the machine, or "" when it is not. The files in it are written 0600, but a
// directory others can write lets them replace the stored session with one of
// their own — after which this CLI reads and writes somebody else's account.
//
// MkdirAll only sets the mode when it creates the directory, so one that was
// already there keeps whatever it had.
func SharedDirWarning() string {
	info, err := os.Stat(Dir())
	if err != nil || !info.IsDir() {
		return ""
	}
	perm := info.Mode().Perm()
	if perm&0o077 == 0 {
		return ""
	}
	return fmt.Sprintf("%s is mode %#o — other users on this machine can reach the stored session; `chmod 700 %s`",
		Dir(), perm, Dir())
}

func statePath(name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("state file name %q must be a simple file name", name)
	}
	return filepath.Join(Dir(), name), nil
}

// Load reads a JSON state file (e.g. the session cache) from the config dir.
func Load(name string, v any) error {
	p, err := statePath(name)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Save writes v as pretty JSON to name in the config dir, 0600 (secrets). The
// write is atomic — temp file, flushed, then os.Rename — so a crash mid-write
// cannot corrupt the session store. Without the flush the rename is still
// atomic for the name, but the file it names can survive a crash empty.
func Save(name string, v any) error {
	path, err := statePath(name)
	if err != nil {
		return err
	}
	if err := ensureDir(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := Dir()
	tmp, err := os.CreateTemp(dir, name+".tmp-*") // opens 0600
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
