package main

// How a command's result reaches the user: the structured formats agents ask
// for, and the fallback to the human view.

import (
	"encoding/json"
	"fmt"
	"os"

	toon "github.com/toon-format/toon-go"
)

// stderrLogf routes diagnostics to stderr, prefixed, so they never mix with
// --json data on stdout.
var stderrLogf = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "netflix: "+format+"\n", args...)
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func emitJSONL(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var rows []json.RawMessage
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &rows); err != nil {
			return err
		}
	} else {
		rows = []json.RawMessage{raw}
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(os.Stdout, string(row)); err != nil {
			return err
		}
	}
	return nil
}

// toonEncode renders v as TOON, routing through JSON first so field names match
// --json exactly.
func toonEncode(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var generic any
	// raw came straight out of json.Marshal, so it decodes.
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "", err
	}
	return toon.MarshalString(generic)
}

func emitTOON(v any) error {
	s, err := toonEncode(v)
	if err != nil || s == "" {
		return err
	}
	_, err = fmt.Fprintln(os.Stdout, s)
	return err
}

// output renders a command's result: the structured format the flags asked for,
// or the human view when they asked for none. Every command ends in this call,
// so the precedence between --toon, --jsonl and --json is decided in one place.
func output(cf *common, v any, human func()) error {
	switch {
	case cf.toon:
		return emitTOON(v)
	case cf.jsonl:
		return emitJSONL(v)
	case cf.jsonOut:
		return emitJSON(v)
	}
	human()
	return nil
}
