package session

import (
	"fmt"
	"strings"
	"testing"
)

func harWith(entries ...string) []byte {
	return []byte(`{"log":{"entries":[` + strings.Join(entries, ",") + `]}}`)
}

func entry(url string, headers ...string) string {
	return fmt.Sprintf(`{"request":{"url":%q,"headers":[%s]}}`, url, strings.Join(headers, ","))
}

func cookieHeader(value string) string {
	return fmt.Sprintf(`{"name":"Cookie","value":%q}`, value)
}

func TestParseHARPrefersTheFreshestSignedInCookie(t *testing.T) {
	data := harWith(
		entry("https://www.netflix.com/browse", cookieHeader("nfvdid=a; NetflixId=old")),
		entry("https://evil.example/steal", cookieHeader("NetflixId=attacker")),
		entry("https://www.netflix.com/title/1", cookieHeader("nfvdid=a; NetflixId=fresh")),
	)
	s, err := ParseHAR(data)
	if err != nil {
		t.Fatalf("ParseHAR: %v", err)
	}
	if !strings.Contains(s.Cookie, "NetflixId=fresh") {
		t.Errorf("cookie = %q, want the last signed-in netflix request", s.Cookie)
	}
	if strings.Contains(s.Cookie, "attacker") {
		t.Error("a cookie from another host must never be imported")
	}
}

// A HAR exported with the download button is stripped of Cookie headers; the
// error has to say how to export one that is not.
func TestParseHARReportsSanitisedExport(t *testing.T) {
	_, err := ParseHAR(harWith(entry("https://www.netflix.com/browse")))
	if err == nil {
		t.Fatal("want an error for a HAR with no Cookie headers")
	}
	if !strings.Contains(err.Error(), "sensitive data") {
		t.Errorf("error %q does not explain how to re-export", err)
	}
}

func TestParseHARReportsWrongSite(t *testing.T) {
	_, err := ParseHAR(harWith(entry("https://www.example.com/", cookieHeader("a=b"))))
	if err == nil || !strings.Contains(err.Error(), "netflix.com") {
		t.Fatalf("error = %v, want one naming netflix.com", err)
	}
}

// Falls back to an anonymous cookie rather than importing nothing: whoami then
// reports it is not signed in, which is a clearer failure.
func TestParseHARAcceptsCookieWithoutNetflixId(t *testing.T) {
	s, err := ParseHAR(harWith(entry("https://www.netflix.com/", cookieHeader("nfvdid=a"))))
	if err != nil {
		t.Fatalf("ParseHAR: %v", err)
	}
	if s.Cookie != "nfvdid=a" {
		t.Errorf("cookie = %q, want the anonymous cookie kept", s.Cookie)
	}
}

func TestParseHARRejectsOversize(t *testing.T) {
	if _, err := ParseHAR(make([]byte, MaxHARBytes+1)); err == nil {
		t.Fatal("want an error for a HAR over the size cap")
	}
}

func TestParseHARRejectsGarbage(t *testing.T) {
	if _, err := ParseHAR([]byte("not json")); err == nil {
		t.Fatal("want an error for a file that is not a HAR")
	}
}
