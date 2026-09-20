package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdOpen opens a title in the system browser, where the signed-in session can
// play it.
func cmdOpen(args []string) error {
	fs, cf := newOutputFlags("open")
	watch := fs.Bool("watch", false, "open the player instead of the title page")
	parseFlags(fs, args)
	raw := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if raw == "" {
		return fmt.Errorf("usage: netflix open <id|url>")
	}
	id, err := client.ParseTitleID(raw)
	if err != nil {
		return err
	}
	cl := newClient(cf)
	page := "title"
	if *watch {
		page = "watch"
	}
	target := fmt.Sprintf("%s/%s/%d", strings.TrimRight(cl.BaseURL, "/"), page, id)
	if err := launchBrowser(target); err != nil {
		return fmt.Errorf("open title: %w", err)
	}
	if done, err := emitStructured(cf, map[string]any{"opened": true, "url": target}); done {
		return err
	}
	fmt.Println(target)
	return nil
}

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
