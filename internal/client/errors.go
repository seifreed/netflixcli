package client

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// APIError carries a non-2xx response so callers can branch on it.
type APIError struct {
	Status     int
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	body := truncate(e.Body, 200)
	if isHTMLBody(e.Body) {
		body = fmt.Sprintf("(HTML error page, %d bytes)", len(e.Body))
	}
	return fmt.Sprintf("netflix: HTTP %d: %s", e.Status, body)
}

// truncate cuts s to n runes, never mid-rune, marking what it dropped.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func isHTMLBody(body string) bool {
	head := strings.ToLower(strings.TrimSpace(body))
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html")
}

// httpStatus reports the HTTP status carried by err when it is (or wraps) an
// *APIError. ok is false for any other error.
func httpStatus(err error) (status int, ok bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status, true
	}
	return 0, false
}

// isNotFound reports whether the gateway answered that the thing asked for does
// not exist, rather than failing for some other reason.
func isNotFound(err error) bool {
	var ge *GraphQLError
	return errors.As(err, &ge) && ge.Extensions.ErrorType == "NOT_FOUND"
}

// ErrNoSession is returned before any request when no cookie is configured.
var ErrNoSession = errors.New("no Netflix session — run `netflix login --from-browser chrome` first")

// ErrSessionRejected is what a 401 or 403 means in practice: the cookie is no
// longer good enough for Netflix. Raw statuses tell the user nothing, so every
// response carrying one is wrapped in this, with the fix in the message.
var ErrSessionRejected = errors.New("netflix refused the session — re-import it with `netflix login --from-browser chrome`")

// rejectsSession reports whether a status means the session was refused rather
// than the request being wrong.
func rejectsSession(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden
}
