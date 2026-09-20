package cookie

import (
	"net/http"
	"testing"
)

func TestMerge(t *testing.T) {
	header := "NetflixId=old; nfvdid=keep; flwssn=drop"
	got := Merge(header, []*http.Cookie{
		{Name: "NetflixId", Value: "fresh"},
		{Name: "SecureNetflixId", Value: "new"},
		{Name: "flwssn", Value: "", MaxAge: -1},
	})
	want := "NetflixId=fresh; SecureNetflixId=new; nfvdid=keep"
	if got != want {
		t.Errorf("Merge = %q, want %q", got, want)
	}
}

// A value no Cookie header could carry must not be spliced into one.
func TestMergeIgnoresUnusableCookies(t *testing.T) {
	got := Merge("a=1", []*http.Cookie{nil, {Name: ""}, {Name: "b", Value: `quo"te`}})
	if got != "a=1" {
		t.Errorf("Merge = %q, want the original header untouched", got)
	}
}
