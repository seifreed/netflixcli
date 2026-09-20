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
	if ctx.User.CountryOfSignup != "ES" {
		t.Errorf("country = %q, want the \\x-escaped blob to have decoded", ctx.User.CountryOfSignup)
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
		"pt-BR": "pt-BR,pt;q=0.9,en;q=0.8",
		// A tag must not appear twice with two different weights.
		"en":    "en",
		"en-GB": "en-GB,en;q=0.9",
	} {
		c := &Client{Lang: lang}
		if got := c.acceptLanguage(); got != want {
			t.Errorf("acceptLanguage(%q) = %q, want %q", lang, got, want)
		}
	}
}

// Every member page carries the bootstrap, so a surface load can supply it and
// save the extra /browse fetch. A page that carries only part of it must not be
// cached, or a later Profiles() would find nothing to list.
func TestAdoptContextRefusesAnIncompletePage(t *testing.T) {
	for name, html := range map[string]string{
		"no bootstrap at all": "<html><body>nothing</body></html>",
		"no client bundle":    bootstrapPage + pageWithProfiles,
		"no profiles":         bootstrapPage,
	} {
		c := New()
		c.adoptContext(html)
		if c.ctx != nil {
			t.Errorf("%s: adopted a bootstrap with profiles=%d bundle=%q",
				name, len(c.ctx.Profiles), c.ctx.BundleURL)
		}
	}
}

// A page carrying all of it is adopted, and context() then costs nothing.
func TestAdoptContextTakesACompletePage(t *testing.T) {
	html := bootstrapPage +
		`<script src="https://assets.nflxext.com/web/ffe/wp/ui/akira/akiraClient.0123456789abcdef.js"></script>` +
		pageWithProfiles
	c := New()
	c.adoptContext(html)
	if c.ctx == nil {
		t.Fatal("a complete page was not adopted")
	}
	if len(c.ctx.Profiles) != 3 || c.ctx.BuildID != "v1a09dd61" {
		t.Errorf("adopted %d profiles for build %q", len(c.ctx.Profiles), c.ctx.BuildID)
	}
	// Adopting again must not replace a bootstrap already in hand.
	before := c.ctx
	c.adoptContext("<html></html>")
	if c.ctx != before {
		t.Error("a later page replaced the bootstrap already held")
	}
}
