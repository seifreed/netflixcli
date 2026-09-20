package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// graphQLClient wires a client to a stub gateway with the bootstrap and query
// manifest already resolved, so only the GraphQL exchange is under test.
func graphQLClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New()
	c.useTransport(srv.Client().Transport)
	c.GraphQLURL = srv.URL
	c.Cookie = "NetflixId=secret"
	c.Lang = "es-ES"
	c.ctx = &shaktiContext{BuildID: "v1a09dd61"}
	c.queries = &queryManifest{Build: "v1a09dd61", Version: 102, Ops: map[string]string{"DemoQuery": "abc-123"}}
	return c
}

func TestGraphQLSendsPersistedOperation(t *testing.T) {
	var body map[string]any
	var headers http.Header
	c := graphQLClient(t, func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Write([]byte(`{"data":{"answer":42}}`))
	})
	var out struct {
		Answer int `json:"answer"`
	}
	if err := c.GraphQL("DemoQuery", map[string]any{"q": "dark"}, &out); err != nil {
		t.Fatalf("GraphQL: %v", err)
	}
	if out.Answer != 42 {
		t.Errorf("answer = %d, want the decoded data", out.Answer)
	}
	if body["operationName"] != "DemoQuery" {
		t.Errorf("operationName = %v", body["operationName"])
	}
	// The document is never sent: only the id the gateway already holds.
	if _, sent := body["query"]; sent {
		t.Error("a persisted operation must not carry the query document")
	}
	persisted := body["extensions"].(map[string]any)["persistedQuery"].(map[string]any)
	if persisted["id"] != "abc-123" || persisted["version"] != float64(102) {
		t.Errorf("persistedQuery = %v, want the manifest's id and version", persisted)
	}
	if headers.Get("x-netflix.context.operation-name") != "DemoQuery" {
		t.Errorf("operation-name header = %q", headers.Get("x-netflix.context.operation-name"))
	}
	if headers.Get("x-netflix.context.app-version") != "v1a09dd61" {
		t.Errorf("app-version header = %q, want the build id", headers.Get("x-netflix.context.app-version"))
	}
	// The cookie is asserted in TestGatewayCookieGoesOnlyToNetflix: this stub is
	// plain HTTP on localhost, which is exactly where it must not go.
}

// The session cookie carries the whole account. It goes to the real gateway and
// nowhere else — NETFLIX_GRAPHQL_URL can name any host, including one that is
// not Netflix and not even HTTPS.
func TestGatewayCookieGoesOnlyToNetflix(t *testing.T) {
	for endpoint, want := range map[string]bool{
		GraphQLEndpoint:                        true,
		"https://web.prod.cloud.netflix.com/g": true,
		"http://web.prod.cloud.netflix.com/g":  false, // downgraded to cleartext
		"https://evil.example/graphql":         false, // not Netflix
		"http://127.0.0.1:9999/graphql":        false, // a local proxy or mock
		"https://netflix.com.evil.example/g":   false, // lookalike host
	} {
		c := graphQLClient(t, func(http.ResponseWriter, *http.Request) {})
		c.GraphQLURL = endpoint
		req, err := c.newGraphQLRequest("DemoQuery", []byte("{}"))
		if err != nil {
			t.Fatalf("newGraphQLRequest(%q): %v", endpoint, err)
		}
		if got := req.Header.Get("cookie") != ""; got != want {
			t.Errorf("%s carried the cookie = %v, want %v", endpoint, got, want)
		}
	}
}

// An endpoint the cookie cannot go to must say so, not fail later as a refused
// session.
func TestUntrustedGatewayIsReported(t *testing.T) {
	var logged []string
	c := graphQLClient(t, func(http.ResponseWriter, *http.Request) {})
	c.Logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	c.GraphQLURL = "http://127.0.0.1:9999/graphql"

	for i := 0; i < 3; i++ {
		if _, err := c.newGraphQLRequest("DemoQuery", []byte("{}")); err != nil {
			t.Fatalf("newGraphQLRequest: %v", err)
		}
	}
	if len(logged) != 1 {
		t.Fatalf("logged %d warnings, want exactly one: %v", len(logged), logged)
	}
	if !strings.Contains(logged[0], "without the session cookie") {
		t.Errorf("warning %q does not say the cookie was withheld", logged[0])
	}
}

// Netflix answers 200 with an errors array; that is a failure, not data.
func TestGraphQLSurfacesErrorsInASuccessfulResponse(t *testing.T) {
	c := graphQLClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"errors":[{"message":"nope","extensions":{"errorType":"BAD_REQUEST"}}],"data":null}`))
	})
	err := c.GraphQL("DemoQuery", nil, nil)
	if err == nil {
		t.Fatal("want an error when the payload carries one")
	}
	if got := err.Error(); got != "netflix graphql: nope (BAD_REQUEST)" {
		t.Errorf("error = %q", got)
	}
}

func TestGraphQLRejectsUnknownOperation(t *testing.T) {
	c := graphQLClient(t, func(http.ResponseWriter, *http.Request) {
		t.Error("an unknown operation must not reach the gateway")
	})
	if err := c.GraphQL("NotInThisBuild", nil, nil); err == nil {
		t.Fatal("want an error for an operation this build does not expose")
	}
}

func TestNewUUIDIsVersion4(t *testing.T) {
	id := newUUID()
	if len(id) != 36 {
		t.Fatalf("uuid = %q, want 36 characters", id)
	}
	if id[14] != '4' {
		t.Errorf("uuid = %q, want version 4", id)
	}
	if newUUID() == id {
		t.Error("two uuids must differ")
	}
}
