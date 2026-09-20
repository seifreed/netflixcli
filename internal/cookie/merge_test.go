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

// The header is sorted so the same jar always produces the same session file,
// and a pair no Cookie header could carry never reaches one.
func TestHeaderIsSortedAndSafe(t *testing.T) {
	jar := map[string]string{"zeta": "1", "alpha": "2", "NetflixId": "3", "bad": `quo"te`}
	want := "NetflixId=3; alpha=2; zeta=1"
	if got := Header(jar); got != want {
		t.Errorf("Header = %q, want %q", got, want)
	}
}

func TestParseSkipsWhatIsNotACookie(t *testing.T) {
	jar := Parse("NetflixId=v%3D3; ; nfvdid=x ;novalue")
	want := map[string]string{"NetflixId": "v%3D3", "nfvdid": "x"}
	if len(jar) != len(want) {
		t.Fatalf("Parse = %v, want %v", jar, want)
	}
	for name, value := range want {
		if jar[name] != value {
			t.Errorf("Parse[%q] = %q, want %q", name, jar[name], value)
		}
	}
}
