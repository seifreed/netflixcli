package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// cmdOpen opens a title in the system browser, where the signed-in session can
// play it.
func cmdOpen(args []string) error {
	fs, cf := newOutputFlags("open")
	watch := fs.Bool("watch", false, "open the player instead of the title page")
	parseFlags(fs, args)
	id, err := titleOperand(fs, "netflix open <id|url>")
	if err != nil {
		return err
	}
	page := "title"
	if *watch {
		page = "watch"
	}
	// Handing a URL to the browser needs no session: building a client here also
	// bootstrapped it and switched profile when one is configured, so `open`
	// failed on a stale cookie it was never going to use.
	target := fmt.Sprintf("%s/%s/%d", strings.TrimRight(baseURL(), "/"), page, id)
	if err := launch(target); err != nil {
		return fmt.Errorf("open title: %w", err)
	}
	return output(cf, map[string]any{"opened": true, "url": target}, func() {
		fmt.Println(target)
	})
}

// launch hands a URL to the system browser. It is a variable so a test can run
// `open` end to end without a browser appearing.
var launch = launchBrowser

func launchBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Run() // #nosec G204 -- command name is fixed by the platform.
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Run() // #nosec G204 -- command name is fixed by the platform.
	default:
		return exec.Command("xdg-open", target).Run() // #nosec G204 -- command name is fixed by the platform.
	}
}
