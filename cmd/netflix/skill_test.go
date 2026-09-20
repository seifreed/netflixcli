package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The skill is what an agent reads before driving this CLI. It says "Do not
// invent commands. If something is not in the reference below, it does not
// exist" — which is only true if the reference and the command table agree.
// These tests hold them together, the way the help and dispatch are held
// together, so the skill cannot drift into documenting something that is gone.

func skillFiles(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	for _, rel := range []string{
		"SKILL.md",
		filepath.Join("references", "cli-reference.md"),
	} {
		path := filepath.Join("..", "..", ".claude", "skills", "netflix-browse", rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		files[rel] = string(data)
	}
	return files
}

// `netflix <word>[ <word>]`, stopping before an operand, a flag or a placeholder.
var skillInvocation = regexp.MustCompile(`netflix ((?:[a-z][a-z-]*)(?:\\\|[a-z]+)*(?: [a-z][a-z-]*(?:\\\|[a-z]+)*)?)`)

// spellings expands `mylist add\|remove` into the commands it names.
func spellings(match string) []string {
	words := strings.Fields(match)
	out := []string{""}
	for _, word := range words {
		var next []string
		for _, alt := range strings.Split(word, `\|`) {
			for _, prefix := range out {
				next = append(next, strings.TrimSpace(prefix+" "+alt))
			}
		}
		out = next
	}
	return out
}

func TestSkillNamesOnlyRealCommands(t *testing.T) {
	checked := 0
	for name, text := range skillFiles(t) {
		for _, m := range skillInvocation.FindAllStringSubmatch(text, -1) {
			for _, spelling := range spellings(m[1]) {
				fields := strings.Fields(spelling)
				if len(fields) == 0 {
					continue
				}
				// `netflix version` and `netflix help` are handled by main, not
				// the table, and are the only two that are.
				if fields[0] == "version" || fields[0] == "help" {
					checked++
					continue
				}
				cmd, _, ok := lookup(fields)
				if !ok {
					t.Errorf("%s names `netflix %s`, which no command dispatches", name, spelling)
					continue
				}
				// A two-word spelling must reach the subcommand, not the parent.
				if len(fields) == 2 && cmd.words() == 1 && !cmd.matches(spelling) {
					t.Errorf("%s names `netflix %s`, but that reaches %q", name, spelling, cmd.name)
				}
				checked++
			}
		}
	}
	if checked < 30 {
		t.Errorf("only %d invocations checked; the pattern is probably not matching", checked)
	}
}

// And the other direction: a command the skill never mentions is one an agent
// will never use. The same expansion is applied, since the reference documents
// a pair like `remind add\|remove` in one row.
func TestSkillCoversEveryCommand(t *testing.T) {
	named := map[string]bool{}
	for _, text := range skillFiles(t) {
		for _, m := range skillInvocation.FindAllStringSubmatch(text, -1) {
			for _, spelling := range spellings(m[1]) {
				named[spelling] = true
			}
		}
	}
	for _, c := range commands() {
		if !named[c.name] {
			t.Errorf("no skill file names `netflix %s`", c.name)
		}
	}
}
