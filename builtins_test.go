package oden

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMachineBuiltinContracts(t *testing.T) {
	app, out, _ := testApp(t, fakeFS{}, "", WithBuiltins())
	app.Add(Command{Name: "task", Aliases: []string{"t"}, Summary: "manage tasks", Category: "Work", Source: "app.go:1", Run: noop})
	for _, args := range [][]string{
		{"commands"}, {"command", "list"}, {"state"}, {"command", "state"},
		{"commands", "--json", "{}", "--yes"},
		{"command", "list", "--json", "{}", "--yes"},
		{"state", "--json", "{}"}, {"command", "state", "--json", "{}"},
	} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			out.Reset()
			app.Args = args
			if err := app.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if args[0] == "commands" {
				var entries []Entry
				if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
					t.Fatal(err)
				}
				for _, e := range entries {
					if e.Name == "task" && e.Summary == "manage tasks" && e.Category == "Work" {
						return
					}
				}
			} else {
				var rows []Row
				if args[0] == "state" || args[1] == "state" {
					dec := json.NewDecoder(out)
					for dec.More() {
						var r Row
						if err := dec.Decode(&r); err != nil {
							t.Fatal(err)
						}
						rows = append(rows, r)
					}
				} else if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
					t.Fatal(err)
				}
				for _, r := range rows {
					if r.Noun == "command" && r.ID == "task" && r.Label == "task  inline  app.go:1" {
						return
					}
				}
			}
			t.Fatalf("missing task command record for %v", args)
		})
	}
	out.Reset()
	app.Args = []string{"command", "show", "t"}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var row struct {
		ID   string      `json:"id"`
		Data CommandData `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &row); err != nil || row.ID != "task" || row.Data.Source != "app.go:1" || row.Data.Category != "Work" {
		t.Fatalf("show: %+v, %v", row, err)
	}
}

func TestAudienceGroupsAndOverrides(t *testing.T) {
	app, out, _ := testApp(t, fakeFS{"/bin": {"mtm-web": 0o755}}, "/bin", WithBuiltins(), WithCategories(map[string]string{"web": "Network"}))
	app.Add(
		Command{Name: "task", Summary: "manage tasks", Category: "Work", Run: noop},
		Command{Name: "export", Category: "Work", Audience: AudienceMachine, Run: noop},
		Command{Name: "catalog", Source: "custom catalog", Run: noop},
		Command{Name: "state", Audience: AudienceHuman, Run: noop},
	)
	app.Args = []string{"help"}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	human, machine, ok := strings.Cut(out.String(), "Machine commands")
	if !ok {
		t.Fatalf("missing audience separation:\n%s", out)
	}
	for _, section := range []struct {
		text  string
		names []string
	}{
		{human, []string{"help", "info", "version", "which", "task", "web", "state"}},
		{machine, []string{"commands", "catalog", "command", "export"}},
	} {
		found := map[string]bool{}
		for _, line := range strings.Split(section.text, "\n") {
			if strings.Contains(line, "(inline)") || strings.Contains(line, "(external)") {
				found[strings.TrimSuffix(strings.Fields(line)[0], ",")] = true
			}
		}
		for _, name := range section.names {
			if !found[name] {
				t.Fatalf("%s in wrong audience group:\n%s", name, out)
			}
		}
	}
	if !strings.Contains(human, "Network:") || !strings.Contains(human, "Work:") || !strings.Contains(machine, "Work:") {
		t.Fatalf("domain categories must remain independent of audience:\n%s", out)
	}
}

func TestHumanInfo(t *testing.T) {
	app, out, _ := testApp(t, fakeFS{}, "", WithBuiltins())
	app.Add(Command{Name: "task", Aliases: []string{"t"}, Summary: "manage tasks", Category: "Work", Source: "app.go:1", Run: noop})
	app.Args = []string{"info", "t"}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mtm task", "manage tasks", "Audience: human", "Category: Work", "app.go:1", "Aliases: t"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	for _, args := range [][]string{{"info", "--human"}, {"info", "-H"}, {"info", "one", "two"}} {
		app.Args = args
		if err := app.Run(context.Background()); ExitCode(err) != 2 {
			t.Fatalf("%v: expected usage exit 2, got %v", args, err)
		}
	}
	out.Reset()
	app.Args = []string{"info", "--", "-h"}
	if err := app.Run(context.Background()); err != nil || !strings.Contains(out.String(), "mtm help") {
		t.Fatalf("literal alias: %v, %s", err, out)
	}
}
