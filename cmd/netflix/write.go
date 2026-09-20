package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdMyListAdd saves a title to the current profile's My List.
func cmdMyListAdd(args []string) error {
	return myListChange(args, "add", func(cl *client.Client, id int) (client.EntityState, error) {
		return cl.AddToMyList(id)
	})
}

// cmdMyListRemove drops a title from the current profile's My List.
func cmdMyListRemove(args []string) error {
	return myListChange(args, "remove", func(cl *client.Client, id int) (client.EntityState, error) {
		return cl.RemoveFromMyList(id)
	})
}

func myListChange(args []string, verb string, change func(*client.Client, int) (client.EntityState, error)) error {
	fs, cf := newCommonFlags("mylist " + verb)
	parseFlags(fs, args)
	raw := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if raw == "" {
		return fmt.Errorf("usage: netflix mylist %s <id|url>", verb)
	}
	id, err := client.ParseTitleID(raw)
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	state, err := change(cl, id)
	if err != nil {
		return err
	}
	if done, err := emitStructured(cf, state); done {
		return err
	}
	if state.InMyList {
		fmt.Printf("%s is in My List\n", state.Title)
	} else {
		fmt.Printf("%s is no longer in My List\n", state.Title)
	}
	return nil
}

// cmdRate sets this profile's thumb rating for a title.
func cmdRate(args []string) error {
	fs, cf := newCommonFlags("rate")
	parseFlags(fs, args)
	rest := fs.Args()
	if len(rest) < 2 {
		return fmt.Errorf("usage: netflix rate <id|url> up|down|love|none")
	}
	id, err := client.ParseTitleID(rest[0])
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	state, err := cl.Rate(id, rest[1])
	if err != nil {
		return err
	}
	if done, err := emitStructured(cf, state); done {
		return err
	}
	fmt.Printf("%s: %s\n", state.Title, strings.ToLower(strings.ReplaceAll(state.ThumbRating, "_", " ")))
	return nil
}

// cmdContinueRemove drops a title from the profile's Continue Watching row.
func cmdContinueRemove(args []string) error {
	fs, cf := newCommonFlags("continue remove")
	parseFlags(fs, args)
	raw := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if raw == "" {
		return fmt.Errorf("usage: netflix continue remove <id|url>")
	}
	id, err := client.ParseTitleID(raw)
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	if err := cl.RemoveFromContinueWatching(id); err != nil {
		return err
	}
	if done, err := emitStructured(cf, map[string]any{"id": id, "removed": true}); done {
		return err
	}
	fmt.Printf("title %d is no longer in Continue Watching\n", id)
	return nil
}

// cmdRemind manages release reminders for titles that are not out yet.
func cmdRemind(args []string) error {
	verb := ""
	if len(args) > 0 {
		verb, args = args[0], args[1:]
	}
	var change func(*client.Client, int) (client.EntityState, error)
	switch verb {
	case "add":
		change = func(cl *client.Client, id int) (client.EntityState, error) { return cl.AddReminder(id) }
	case "remove", "rm":
		change = func(cl *client.Client, id int) (client.EntityState, error) { return cl.RemoveReminder(id) }
	default:
		return fmt.Errorf("usage: netflix remind add|remove <id|url>")
	}
	fs, cf := newCommonFlags("remind " + verb)
	parseFlags(fs, args)
	raw := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if raw == "" {
		return fmt.Errorf("usage: netflix remind %s <id|url>", verb)
	}
	id, err := client.ParseTitleID(raw)
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	state, err := change(cl, id)
	if err != nil {
		return err
	}
	if done, err := emitStructured(cf, state); done {
		return err
	}
	if state.Reminder {
		fmt.Printf("%s: Netflix will remind you when it arrives\n", state.Title)
	} else {
		fmt.Printf("%s: reminder removed\n", state.Title)
	}
	return nil
}
