package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// cmdMyListAdd saves a title to the current profile's My List.
func cmdMyListAdd(args []string) error {
	return myListChange(args, "add", func(cl *client.Client, id int) (client.EntityState, error) {
		return cl.Library.AddToMyList(id)
	})
}

// cmdMyListRemove drops a title from the current profile's My List.
func cmdMyListRemove(args []string) error {
	return myListChange(args, "remove", func(cl *client.Client, id int) (client.EntityState, error) {
		return cl.Library.RemoveFromMyList(id)
	})
}

func myListChange(args []string, verb string, change func(*client.Client, int) (client.EntityState, error)) error {
	fs, cf := newCommonFlags("mylist " + verb)
	parseFlags(fs, args)
	id, err := titleOperand(fs, fmt.Sprintf("netflix mylist %s <id|url>", verb))
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
	return output(cf, state, func() {
		if state.InMyList {
			fmt.Printf("%s is in My List\n", state.Title)
			return
		}
		fmt.Printf("%s is no longer in My List\n", state.Title)
	})
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
	state, err := cl.Library.Rate(id, rest[1])
	if err != nil {
		return err
	}
	return output(cf, state, func() {
		fmt.Printf("%s: %s\n", state.Title, strings.ToLower(strings.ReplaceAll(state.ThumbRating, "_", " ")))
	})
}

// cmdContinueRemove drops a title from the profile's Continue Watching row.
func cmdContinueRemove(args []string) error {
	fs, cf := newCommonFlags("continue remove")
	parseFlags(fs, args)
	id, err := titleOperand(fs, "netflix continue remove <id|url>")
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	if err := cl.Library.RemoveFromContinueWatching(id); err != nil {
		return err
	}
	return output(cf, map[string]any{"id": id, "removed": true}, func() {
		fmt.Printf("title %d is no longer in Continue Watching\n", id)
	})
}

// cmdRemindAdd asks Netflix to remind this profile when a title arrives.
func cmdRemindAdd(args []string) error {
	return remindChange(args, "add", func(cl *client.Client, id int) (client.EntityState, error) {
		return cl.Library.AddReminder(id)
	})
}

// cmdRemindRemove drops a title's release reminder.
func cmdRemindRemove(args []string) error {
	return remindChange(args, "remove", func(cl *client.Client, id int) (client.EntityState, error) {
		return cl.Library.RemoveReminder(id)
	})
}

func remindChange(args []string, verb string, change func(*client.Client, int) (client.EntityState, error)) error {
	fs, cf := newCommonFlags("remind " + verb)
	parseFlags(fs, args)
	id, err := titleOperand(fs, fmt.Sprintf("netflix remind %s <id|url>", verb))
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
	return output(cf, state, func() {
		// The reminder mutations answer without a title, so the id is what there is
		// to name, and the two flags are reported exactly as they came back rather
		// than narrated: asking to be reminded about an already-available title also
		// files it in My List, and the response does not distinguish that from a
		// title that was in My List already.
		fmt.Printf("title %d — reminder: %s · My List: %s\n", state.ID, yesNo(state.Reminder), yesNo(state.InMyList))
	})
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
