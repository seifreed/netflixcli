package browser

import "testing"

// The CDP endpoint is a debugging port with full control of the user's browser,
// so only a local one may ever be dialled.
func TestNewRejectsNonLocalEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"http://evil.example:9222",
		"https://198.51.100.10:9222",
		"ws://127.0.0.1:9222",
		"http://user:pass@127.0.0.1:9222",
		"not a url at all",
		"http://",
	} {
		if _, err := New(endpoint); err == nil {
			t.Errorf("New(%q) was accepted, want a refusal", endpoint)
		}
	}
}

func TestNewAcceptsLoopback(t *testing.T) {
	for _, endpoint := range []string{"", "http://127.0.0.1:9222", "http://localhost:9222", "http://[::1]:9222"} {
		if _, err := New(endpoint); err != nil {
			t.Errorf("New(%q): %v", endpoint, err)
		}
	}
}
