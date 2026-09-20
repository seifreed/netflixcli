package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testClient points a Client at a stub server, with the uTLS transport replaced
// by the stub's plain-HTTP one.
func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	c.useTransport(srv.Client().Transport)
	return c, srv
}

func TestGetTextSendsWebAppHeaders(t *testing.T) {
	var got http.Header
	c, srv := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Write([]byte("<html></html>"))
	})
	c.Lang = "ca-ES"
	if _, err := c.getText(srv.URL + "/browse"); err != nil {
		t.Fatalf("getText: %v", err)
	}
	if !strings.Contains(got.Get("user-agent"), "Chrome/") {
		t.Errorf("user-agent = %q, want a Chrome one", got.Get("user-agent"))
	}
	if want := "ca-ES,ca;q=0.9,en;q=0.8"; got.Get("accept-language") != want {
		t.Errorf("accept-language = %q, want %q", got.Get("accept-language"), want)
	}
}

// The session cookie is the user's account: it must only ever go to Netflix over
// https, never to the stub host or any other origin.
func TestCookieIsWithheldFromForeignHosts(t *testing.T) {
	var got string
	c, srv := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("cookie")
		w.Write([]byte("<html></html>"))
	})
	c.Cookie = "NetflixId=secret"
	if _, err := c.getText(srv.URL + "/browse"); err != nil {
		t.Fatalf("getText: %v", err)
	}
	if got != "" {
		t.Fatalf("sent cookie %q to %s, want none", got, srv.URL)
	}
	if trustedCookieRequest("http://www.netflix.com/browse") {
		t.Error("plain http must not carry the session cookie")
	}
	if !trustedCookieRequest("https://web.prod.cloud.netflix.com/graphql") {
		t.Error("the GraphQL host is a netflix.com subdomain and must carry it")
	}
}

func TestGetTextRefusesOtherOrigins(t *testing.T) {
	c, _ := testClient(t, func(http.ResponseWriter, *http.Request) {})
	if _, err := c.getText("https://evil.example/browse"); err == nil {
		t.Fatal("want a refusal for a URL outside the configured host")
	}
}

func TestGetTextRetriesThrottling(t *testing.T) {
	var calls atomic.Int32
	c, srv := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("<html>ok</html>"))
	})
	body, err := c.getText(srv.URL + "/browse")
	if err != nil {
		t.Fatalf("getText: %v", err)
	}
	if body != "<html>ok</html>" {
		t.Errorf("body = %q, want the retried response", body)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("made %d requests, want a retry after the 429", n)
	}
}

func TestGetTextGivesUpAfterMaxRetries(t *testing.T) {
	var calls atomic.Int32
	c, srv := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if _, err := c.getText(srv.URL + "/browse"); err == nil {
		t.Fatal("want an error once the retries run out")
	}
	if n := calls.Load(); n != maxRetries+1 {
		t.Errorf("made %d requests, want %d", n, maxRetries+1)
	}
}

// A refused session is the commonest failure once a cookie ages out. Reporting
// it as a bare HTTP status leaves the user with nothing to do about it.
func TestRefusedSessionIsActionable(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		c, srv := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		})
		_, err := c.getText(srv.URL + "/browse")
		if !errors.Is(err, ErrSessionRejected) {
			t.Errorf("HTTP %d gave %v, want it to wrap ErrSessionRejected", status, err)
		}
		if !strings.Contains(err.Error(), "login --from-browser") {
			t.Errorf("HTTP %d message %q does not say how to fix it", status, err)
		}
	}
}

// Any other failure keeps its status for callers that branch on it.
func TestOtherStatusesStayAPIErrors(t *testing.T) {
	c, srv := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.getText(srv.URL + "/browse")
	status, ok := httpStatus(err)
	if !ok || status != http.StatusNotFound {
		t.Errorf("status = %d (ok=%v), want 404 preserved", status, ok)
	}
	if errors.Is(err, ErrSessionRejected) {
		t.Error("a 404 must not be reported as a refused session")
	}
}

// An HTML error page is megabytes of markup; the message must summarise it.
func TestAPIErrorSummarisesHTMLBodies(t *testing.T) {
	err := &APIError{Status: 403, Body: "<!DOCTYPE html><html>" + strings.Repeat("x", 5000)}
	if msg := err.Error(); strings.Contains(msg, "xxxx") || !strings.Contains(msg, "HTML error page") {
		t.Errorf("message = %q, want an HTML summary", msg)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("3"); got != 3*time.Second {
		t.Errorf("parseRetryAfter(3) = %v, want 3s", got)
	}
	if got := parseRetryAfter(""); got != -1 {
		t.Errorf("parseRetryAfter(\"\") = %v, want -1 (absent)", got)
	}
	if got := parseRetryAfter("nonsense"); got != -1 {
		t.Errorf("parseRetryAfter(nonsense) = %v, want -1", got)
	}
	if got := parseRetryAfter(time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)); got != 0 {
		t.Errorf("a past Retry-After date = %v, want 0", got)
	}
}

func TestCapWait(t *testing.T) {
	if got := capWait(time.Hour); got != maxRetryWait {
		t.Errorf("capWait(1h) = %v, want the cap %v", got, maxRetryWait)
	}
	if got := capWait(-time.Second); got != 0 {
		t.Errorf("capWait(-1s) = %v, want 0", got)
	}
}

func TestGraphQLWithoutSession(t *testing.T) {
	c := New()
	if err := c.GraphQL("SearchPageQueryResults", nil, nil); err == nil {
		t.Fatal("want an error when no session is configured")
	}
}

// The user agent is applied by the transport, so a request built anywhere in
// the package carries it — including the ones that do not go through newReq.
func TestEveryRequestCarriesTheUserAgent(t *testing.T) {
	var seen []string
	c, srv := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("user-agent"))
		w.Write([]byte("{}"))
	})
	req, err := http.NewRequest("GET", srv.URL+"/anything", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := c.HTTP.Do(req); err != nil {
		t.Fatalf("do: %v", err)
	}
	if len(seen) != 1 || !strings.Contains(seen[0], "Chrome/") {
		t.Errorf("user-agent = %q, want the Chrome one the fingerprint claims", seen)
	}
}

// A caller that sets its own user agent keeps it.
func TestExplicitUserAgentIsNotOverwritten(t *testing.T) {
	var seen string
	c, srv := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("user-agent")
		w.Write([]byte("{}"))
	})
	req, _ := http.NewRequest("GET", srv.URL+"/anything", nil)
	req.Header.Set("user-agent", "custom/1.0")
	if _, err := c.HTTP.Do(req); err != nil {
		t.Fatalf("do: %v", err)
	}
	if seen != "custom/1.0" {
		t.Errorf("user-agent = %q, want the caller's", seen)
	}
}
