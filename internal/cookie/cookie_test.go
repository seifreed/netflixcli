package cookie

import "testing"

func TestIsNetflixHost(t *testing.T) {
	for host, want := range map[string]bool{
		"netflix.com":                true,
		"www.netflix.com":            true,
		"web.prod.cloud.netflix.com": true,
		"netflix.com.evil.example":   false,
		"evilnetflix.com":            false,
		"":                           false,
	} {
		if got := IsNetflixHost(host); got != want {
			t.Errorf("IsNetflixHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestLooksAuthenticated(t *testing.T) {
	if !LooksAuthenticated("nfvdid=x; NetflixId=v%3D3%26ct%3D1; SecureNetflixId=y") {
		t.Error("a header carrying NetflixId should look authenticated")
	}
	if LooksAuthenticated("nfvdid=x; flwssn=y") {
		t.Error("a header without NetflixId should not look authenticated")
	}
}

func TestValidHeaderRejectsOversize(t *testing.T) {
	big := make([]byte, MaxHeaderBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	if ValidHeader(string(big)) {
		t.Error("an oversized header must be rejected")
	}
	if ValidHeader("") {
		t.Error("an empty header must be rejected")
	}
}
