package main

import (
	"strings"
	"testing"
)

// The point of the table is that help and dispatch cannot disagree. These tests
// fail if a command is added to one and not the other.
func TestEveryDispatchableCommandIsDocumented(t *testing.T) {
	text := usageText()
	for _, c := range commands() {
		if !strings.Contains(text, c.name) {
			t.Errorf("command %q is dispatchable but absent from the help", c.name)
		}
	}
}

// Every documented row must dispatch to itself — including a subcommand row,
// which once documented a name that only its parent's hand-written switch knew
// how to reach.
func TestEveryDocumentedCommandResolves(t *testing.T) {
	for _, c := range helpRows() {
		if c.run == nil {
			t.Errorf("help documents %q but the row has no run func", c.name)
			continue
		}
		got, rest, ok := lookup(strings.Fields(c.name))
		if !ok {
			t.Errorf("help documents %q but nothing dispatches it", c.name)
			continue
		}
		if got.name != c.name {
			t.Errorf("%q dispatches to %q instead of itself", c.name, got.name)
		}
		if len(rest) != 0 {
			t.Errorf("%q left %v unconsumed", c.name, rest)
		}
	}
}

// The longest spelling wins, so a parent row and its subcommands coexist.
func TestLookupTakesTheLongestSpelling(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
		rest []string
	}{
		{[]string{"mylist"}, "mylist", nil},
		{[]string{"my-list", "--json"}, "mylist", []string{"--json"}},
		{[]string{"mylist", "add", "123"}, "mylist add", []string{"123"}},
		{[]string{"mylist", "rm", "123"}, "mylist remove", []string{"123"}},
		{[]string{"continue"}, "continue", nil},
		{[]string{"continue-watching"}, "continue", nil},
		{[]string{"continue", "remove", "9"}, "continue remove", []string{"9"}},
		{[]string{"remind", "add", "9"}, "remind add", []string{"9"}},
		{[]string{"profile"}, "profiles", nil},
		{[]string{"profile", "list"}, "profiles", nil},
		{[]string{"profile", "use", "kids"}, "profile use", []string{"kids"}},
		// A flag or operand that happens to follow must not be eaten as a
		// subcommand of a command that has none.
		{[]string{"search", "add"}, "search", []string{"add"}},
	} {
		got, rest, ok := lookup(tc.args)
		if !ok {
			t.Errorf("lookup(%v) found nothing", tc.args)
			continue
		}
		if got.name != tc.want {
			t.Errorf("lookup(%v) = %q, want %q", tc.args, got.name, tc.want)
		}
		if strings.Join(rest, " ") != strings.Join(tc.rest, " ") {
			t.Errorf("lookup(%v) left %v, want %v", tc.args, rest, tc.rest)
		}
	}
	if _, _, ok := lookup([]string{"nonsense"}); ok {
		t.Error("lookup resolved a command that does not exist")
	}
	if _, _, ok := lookup(nil); ok {
		t.Error("lookup resolved something from no arguments")
	}
}

func TestHelpGroupsAreOrderedAndTitled(t *testing.T) {
	text := usageText()
	last := -1
	for _, title := range []string{
		groupTitles[groupRead], groupTitles[groupWrite],
		groupTitles[groupAccount], groupTitles[groupSession],
	} {
		at := strings.Index(text, title)
		if at < 0 {
			t.Fatalf("help is missing the %q section", title)
		}
		if at < last {
			t.Errorf("section %q is out of order", title)
		}
		last = at
	}
}

// Help is read in a terminal; a row that overflows its column is unreadable.
func TestHelpRowsFitTheColumn(t *testing.T) {
	for _, c := range helpRows() {
		if spelled := invocation(c); len(spelled) > helpNameWidth {
			t.Errorf("help row %q is %d characters, over the %d-column width",
				spelled, len(spelled), helpNameWidth)
		}
	}
}

func TestCommandNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range commandNames() {
		if seen[name] {
			t.Errorf("%q is registered twice", name)
		}
		seen[name] = true
	}
}

// `netflix remind` is not a command, but reporting it as unknown would be a lie.
// The hint is read off the table, so a new subcommand appears in it for free.
func TestParentCommandsNameTheirSubcommands(t *testing.T) {
	for parent, want := range map[string]string{
		"remind":   "add remove",
		"mylist":   "add remove",
		"continue": "remove",
		"profile":  "use",
		"search":   "",
	} {
		if got := strings.Join(subcommandsOf(parent), " "); got != want {
			t.Errorf("subcommandsOf(%q) = %q, want %q", parent, got, want)
		}
	}
}
