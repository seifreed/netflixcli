package session

import (
	"fmt"

	"github.com/seifreed/netflixcli/internal/config"
	"github.com/seifreed/netflixcli/internal/cookie"
)

const sessionFile = "session.json"

// Session is the machine-managed auth cache: the browser cookie carrying the
// Netflix membership (NetflixId / SecureNetflixId). Written by login/import-har/
// set-cookie, read by every command.
type Session struct {
	Cookie string `json:"cookie"`
}

// LoadSession reads the cached session; a missing file yields a zero Session.
func LoadSession() Session {
	var s Session
	_ = config.Load(sessionFile, &s)
	return s
}

// SaveSession persists the session cache (0600).
func SaveSession(s Session) error {
	if s.Cookie != "" && !cookie.ValidHeader(s.Cookie) {
		return fmt.Errorf("invalid Cookie header")
	}
	return config.Save(sessionFile, s)
}
