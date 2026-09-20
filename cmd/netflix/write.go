package main

import (
	"fmt"
	"strings"

	"github.com/seifreed/netflixcli/internal/client"
)

// libraryWrite is a write that names one title and answers with its new state —
// the shape of every such method on client.Library, so callers pass the method
// itself rather than a closure around it.
type libraryWrite func(*client.Library, int) (client.EntityState, error)

// entityWrite runs one of those: it reads the title operand, builds the client,
// applies the write and renders what came back. Every such command has this
// shape; only the write and how its result reads differ, so they say only that.
func entityWrite(args []string, name string, apply libraryWrite, human func(client.EntityState)) error {
	fs, cf := newCommonFlags(name)
	parseFlags(fs, args)
	id, err := titleOperand(fs, "netflix "+name+" <id|url>")
	if err != nil {
		return err
	}
	cl, err := newClient(cf)
	if err != nil {
		return err
	}
	state, err := apply(cl.Library, id)
	if err != nil {
		return err
	}
	return output(cf, state, func() { human(state) })
}

// cmdMyListAdd saves a title to the current profile's My List.
func cmdMyListAdd(args []string) error {
	return entityWrite(args, "mylist add", (*client.Library).AddToMyList, printMyListState)
}

// cmdMyListRemove drops a title from the current profile's My List.
func cmdMyListRemove(args []string) error {
	return entityWrite(args, "mylist remove", (*client.Library).RemoveFromMyList, printMyListState)
}

func printMyListState(state client.EntityState) {
	if state.InMyList {
		fmt.Printf("%s is in My List\n", state.Title)
		return
	}
	fmt.Printf("%s is no longer in My List\n", state.Title)
}

// cmdRate sets this profile's thumb rating for a title.
func cmdRate(args []string) error {
	fs, cf := newCommonFlags("rate")
	parseFlags(fs, args)
	rest := fs.Args()
	if len(rest) < 2 {
		return usagef("usage: netflix rate <id|url> up|down|love|none")
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
	return entityWrite(args, "remind add", (*client.Library).AddReminder, printReminderState)
}

// cmdRemindRemove drops a title's release reminder.
func cmdRemindRemove(args []string) error {
	return entityWrite(args, "remind remove", (*client.Library).RemoveReminder, printReminderState)
}

// printReminderState reports the two flags exactly as they came back rather than
// narrating them. The reminder mutations answer without a title, so the id is
// what there is to name, and asking to be reminded about an already-available
// title also files it in My List — which the response does not distinguish from
// a title that was in My List already.
func printReminderState(state client.EntityState) {
	fmt.Printf("title %d — reminder: %s · My List: %s\n", state.ID, yesNo(state.Reminder), yesNo(state.InMyList))
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
