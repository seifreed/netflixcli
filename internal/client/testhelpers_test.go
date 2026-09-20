package client

import (
	"encoding/json"
	"testing"
)

func mustUnmarshal(t *testing.T, raw string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
}
