package client

import "testing"

// Netflix serialises the bootstrap with JavaScript \xNN escapes, which are not
// valid JSON; the parser has to rewrite them before decoding.
const bootstrapPage = `<html><body><script>window.netflix = window.netflix || {};` +
	`netflix.reactContext = {"models":{"serverDefs":{"data":{"BUILD_IDENTIFIER":"v1a09dd61",` +
	`"originalUrl":"\x2Fbrowse"}},"userInfo":{"data":{"name":"Ada","membershipStatus":"CURRENT_MEMBER",` +
	`"countryOfSignup":"ES","numProfiles":4,"authURL":"c1.token\x3D\x3D"}}}};</script></body></html>`

func TestParseReactContext(t *testing.T) {
	ctx, err := parseReactContext(bootstrapPage)
	if err != nil {
		t.Fatalf("parseReactContext: %v", err)
	}
	if ctx.BuildID != "v1a09dd61" {
		t.Errorf("build id = %q, want v1a09dd61", ctx.BuildID)
	}
	if ctx.AuthURL != "c1.token==" {
		t.Errorf("authURL = %q, want the \\x escapes decoded", ctx.AuthURL)
	}
	if ctx.User.Name != "Ada" || ctx.User.NumProfiles != 4 {
		t.Errorf("user = %+v, want Ada with 4 profiles", ctx.User)
	}
}

func TestParseReactContextRejectsLoggedOutPage(t *testing.T) {
	if _, err := parseReactContext("<html><body>sign in</body></html>"); err == nil {
		t.Fatal("want an error when the page carries no bootstrap")
	}
}

func TestAcceptLanguage(t *testing.T) {
	for lang, want := range map[string]string{
		"":      "es-ES,es;q=0.9,en;q=0.8",
		"es-ES": "es-ES,es;q=0.9,en;q=0.8",
		"en":    "en;q=0.9,en;q=0.8",
	} {
		c := &Client{Lang: lang}
		if got := c.acceptLanguage(); got != want {
			t.Errorf("acceptLanguage(%q) = %q, want %q", lang, got, want)
		}
	}
}
