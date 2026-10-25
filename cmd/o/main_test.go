package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unixsuperhero/oden"
)

// o has no inline commands beyond the builtins: o tui and o proj are
// o-tui and o-proj on PATH, run with the remaining words and ODEN_PARENT=o.
func TestOToolsAreOPrefixedExternals(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"o-tui", "o-proj"} {
		script := "#!/bin/sh\necho \"" + name + " $* parent=$ODEN_PARENT\"\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"tui", "--smoke"}, "o-tui --smoke parent=o\n"},
		{[]string{"proj", "new", "x"}, "o-proj new x parent=o\n"},
		{[]string{"pr", "check"}, "o-proj check parent=o\n"},
	} {
		var out bytes.Buffer
		app := newApp(c.args, oden.WithPath(dir), oden.WithEnv([]string{}), oden.WithStdio(strings.NewReader(""), &out, &out))
		if err := app.Run(context.Background()); err != nil || out.String() != c.want {
			t.Fatalf("o %v: %v %q", c.args, err, out.String())
		}
	}
}

func fakeTool(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho "+name+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func runO(t *testing.T, path string, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	app := newApp(args, oden.WithPath(path), oden.WithEnv([]string{}), oden.WithStdio(strings.NewReader(""), &out, &errOut))
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("o %v: %v\n%s", args, err, errOut.String())
	}
	return out.String()
}

// o catalog answers schema 1 with one noun, command: list (rows command),
// show (positional name) and state, the catalog's state; no bindings, no
// pages.
func TestOCatalogIsSchemaOneWithTheCommandNoun(t *testing.T) {
	var doc struct {
		Schema   int    `json:"schema"`
		Name     string `json:"name"`
		Version  *string
		State    string `json:"state"`
		Bindings []any  `json:"bindings"`
		Pages    []any  `json:"pages"`
		Nouns    []struct {
			Name  string `json:"name"`
			Verbs []struct {
				Name       string                     `json:"name"`
				Kind       string                     `json:"kind"`
				Rows       string                     `json:"rows"`
				Positional []string                   `json:"positional"`
				Params     map[string]json.RawMessage `json:"params"`
			} `json:"verbs"`
		} `json:"nouns"`
	}
	out := runO(t, t.TempDir(), "catalog")
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("catalog: %v\n%s", err, out)
	}
	if doc.Schema != 1 || doc.Name != "o" || doc.State != "command.state" || doc.Bindings == nil || len(doc.Bindings) != 0 || doc.Pages == nil || len(doc.Pages) != 0 {
		t.Fatalf("catalog doc %+v", doc)
	}
	if !strings.Contains(out, `"version": `) {
		t.Fatalf("catalog has no version field:\n%s", out)
	}
	if len(doc.Nouns) != 1 || doc.Nouns[0].Name != "command" || len(doc.Nouns[0].Verbs) != 3 {
		t.Fatalf("nouns %+v", doc.Nouns)
	}
	v := doc.Nouns[0].Verbs
	if v[0].Name != "list" || v[0].Rows != "command" || v[0].Kind != "application" ||
		v[1].Name != "show" || len(v[1].Positional) != 1 || v[1].Positional[0] != "name" || v[1].Params["name"] == nil ||
		v[2].Name != "state" {
		t.Fatalf("verbs %+v", v)
	}
}

// o state is one JSON Line per catalog entry, tagged command: an external
// o-proj, the first o-x on PATH with the later one in its shadows, and the
// builtins; o command state says the same and o command show one row.
func TestOStateRowsForAFakePath(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	proj := fakeTool(t, a, "o-proj")
	x := fakeTool(t, a, "o-x")
	shadowedX := fakeTool(t, b, "o-x")
	path := a + string(os.PathListSeparator) + b
	out := runO(t, path, "state")
	if alt := runO(t, path, "command", "state"); alt != out {
		t.Fatalf("command state differs:\n%s\nvs\n%s", alt, out)
	}
	type data struct {
		Kind    string       `json:"kind"`
		Path    string       `json:"path"`
		Source  string       `json:"source"`
		Aliases []string     `json:"aliases"`
		Shadows []oden.Claim `json:"shadows"`
		Prefix  string       `json:"prefix"`
	}
	type row struct {
		Noun  string `json:"noun"`
		ID    string `json:"id"`
		Label string `json:"label"`
		Data  data   `json:"data"`
	}
	rows := map[string]row{}
	var order []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var r row
		if err := json.Unmarshal([]byte(line), &r); err != nil || r.Noun != "command" {
			t.Fatalf("line %q: %v", line, err)
		}
		rows[r.ID] = r
		order = append(order, r.ID)
	}
	want := []string{"catalog", "command", "commands", "help", "info", "proj", "state", "version", "which", "x"}
	if strings.Join(order, " ") != strings.Join(want, " ") {
		t.Fatalf("ids %v, want %v", order, want)
	}
	if r := rows["proj"]; r.Label != "proj  external  "+proj || r.Data.Kind != "external" || r.Data.Path != proj || r.Data.Prefix != "o" || len(r.Data.Shadows) != 0 {
		t.Fatalf("proj %+v", r)
	}
	if r := rows["x"]; r.Data.Path != x || len(r.Data.Shadows) != 1 || r.Data.Shadows[0].Source != shadowedX || r.Data.Shadows[0].Kind != oden.KindExternal {
		t.Fatalf("x %+v", r)
	}
	if r := rows["help"]; r.Data.Kind != "inline" || r.Data.Path != "" || strings.Join(r.Data.Aliases, ",") != "-h,--help" || !strings.HasPrefix(r.Label, "help  inline  oden/builtins.go:") {
		t.Fatalf("help %+v", r)
	}
	show := runO(t, path, "command", "show", "pr")
	if !strings.Contains(show, `"id": "proj"`) || !strings.Contains(show, `"path": "`+proj+`"`) {
		t.Fatalf("show:\n%s", show)
	}
	var listed []row
	list := runO(t, path, "command", "list")
	if err := json.Unmarshal([]byte(list), &listed); err != nil {
		t.Fatalf("list: %v\n%s", err, list)
	}
	byID := map[string]row{}
	for _, r := range listed {
		byID[r.ID] = r
	}
	if byID["proj"].Data.Path != proj || byID["x"].Data.Path != x || byID["x"].Data.Shadows[0].Source != shadowedX {
		t.Fatalf("listed external commands: %+v", byID)
	}
}
