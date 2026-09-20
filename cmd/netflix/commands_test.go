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
		if c.run == nil || c.name == "profile" {
			continue
		}
		if !strings.Contains(text, c.name) {
			t.Errorf("command %q is dispatchable but absent from the help", c.name)
		}
	}
}

func TestEveryDocumentedCommandResolves(t *testing.T) {
	for _, c := range helpRows() {
		// A help row for a subcommand documents "parent sub"; the parent is what
		// dispatches.
		name := c.name
		if parent, _, isSub := strings.Cut(name, " "); isSub {
			name = parent
		}
		if _, ok := lookup(name); !ok {
			t.Errorf("help documents %q but no command dispatches %q", c.name, name)
		}
	}
}

func TestLookupResolvesAliases(t *testing.T) {
	for name, want := range map[string]string{
		"mylist":            "mylist",
		"my-list":           "mylist",
		"continue":          "continue",
		"continue-watching": "continue",
	} {
		got, ok := lookup(name)
		if !ok {
			t.Errorf("lookup(%q) found nothing", name)
			continue
		}
		if got.name != want {
			t.Errorf("lookup(%q) = %q, want %q", name, got.name, want)
		}
	}
	if _, ok := lookup("nonsense"); ok {
		t.Error("lookup resolved a command that does not exist")
	}
	// Help-only rows document a subcommand; they must not be dispatchable on
	// their own, or `netflix "mylist add"` would look like a command.
	if _, ok := lookup("mylist add"); ok {
		t.Error("a help-only row must not be dispatchable")
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
