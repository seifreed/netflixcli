package client

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GraphQLEndpoint is the gateway the Netflix web app queries. It is a separate
// host from www.netflix.com but shares the netflix.com cookie jar.
const GraphQLEndpoint = "https://web.prod.cloud.netflix.com/graphql"

// uiFlavor identifies the web UI the persisted queries belong to.
const uiFlavor = "akira"

// GraphQLError is one error entry returned by the gateway.
type GraphQLError struct {
	Message    string `json:"message"`
	Extensions struct {
		ErrorType      string `json:"errorType"`
		Classification string `json:"classification"`
	} `json:"extensions"`
}

func (e *GraphQLError) Error() string {
	if e.Extensions.ErrorType != "" {
		return fmt.Sprintf("netflix graphql: %s (%s)", e.Message, e.Extensions.ErrorType)
	}
	return "netflix graphql: " + e.Message
}

// GraphQL runs one persisted operation by name and decodes its `data` into out.
// The operation must exist in the current build's query manifest.
func (c *Client) GraphQL(op string, variables map[string]any, out any) error {
	manifest, err := c.manifest()
	if err != nil {
		return err
	}
	id, ok := manifest.id(op)
	if !ok {
		return fmt.Errorf("netflix build %s does not expose the GraphQL operation %q", manifest.Build, op)
	}
	body, err := json.Marshal(map[string]any{
		"operationName": op,
		"variables":     variables,
		"extensions": map[string]any{
			"persistedQuery": map[string]any{"id": id, "version": manifest.Version},
		},
	})
	if err != nil {
		return fmt.Errorf("encode %s variables: %w", op, err)
	}
	req, err := c.newGraphQLRequest(op, body)
	if err != nil {
		return err
	}
	raw, err := c.do(req)
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []*GraphQLError `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode %s response: %w", op, err)
	}
	if len(envelope.Errors) > 0 {
		return envelope.Errors[0]
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *Client) newGraphQLRequest(op string, body []byte) (*http.Request, error) {
	if c.Cookie == "" {
		return nil, ErrNoSession
	}
	ctx, err := c.context()
	if err != nil {
		return nil, err
	}
	endpoint := c.GraphQLURL
	if endpoint == "" {
		endpoint = GraphQLEndpoint
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "*/*")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept-language", c.acceptLanguage())
	req.Header.Set("user-agent", c.UserAgent)
	req.Header.Set("origin", BaseURL)
	req.Header.Set("referer", BaseURL+"/")
	req.Header.Set("cookie", c.Cookie)
	req.Header.Set("x-netflix.context.app-version", ctx.BuildID)
	req.Header.Set("x-netflix.context.locales", c.locale())
	req.Header.Set("x-netflix.context.operation-name", op)
	req.Header.Set("x-netflix.context.ui-flavor", uiFlavor)
	req.Header.Set("x-netflix.request.attempt", "1")
	req.Header.Set("x-netflix.request.client.context", `{"appstate":"foreground"}`)
	req.Header.Set("x-netflix.request.id", randomHex(16))
	return req, nil
}

func (c *Client) locale() string {
	if l := strings.TrimSpace(c.Lang); l != "" {
		return l
	}
	return "es-ES"
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(b)
}

// newUUID returns a random RFC 4122 v4 UUID. Netflix's search page groups the
// queries of one typing session under such an id.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
