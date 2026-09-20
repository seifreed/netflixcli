package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/seifreed/netflixcli/internal/client"
	"github.com/seifreed/netflixcli/internal/session"
)

func main() {
	cl := client.New()
	cl.Cookie = session.LoadSession().Cookie
	cl.Logf = func(f string, a ...any) { fmt.Fprintf(os.Stderr, "log: "+f+"\n", a...) }
	var out json.RawMessage
	err := cl.GraphQL("DetailModal", map[string]any{
		"unifiedEntityId":         "Video:70095139",
		"videoId":                 70095139,
		"checkLinearChannel":      true,
		"opaqueImageFormat":       "WEBP",
		"transparentImageFormat":  "WEBP",
		"videoMerchEnabled":       true,
		"fetchPromoVideoOverride": false,
		"hasPromoVideoOverride":   false,
		"promoVideoId":            0,
		"videoMerchContext":       "BROWSE",
		"isLiveEpisodic":          false,
		"includeCroppedLogo":      false,
		"artworkContext":          map[string]any{},
		"textEvidenceUiContext":   "ODP",
	}, &out)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	os.WriteFile("/tmp/nf-detailmodal.json", out, 0o600)
	fmt.Println("bytes", len(out))
}
