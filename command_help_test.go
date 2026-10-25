package oden

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestCommandGroupHelp(t *testing.T) {
	for _, args := range [][]string{
		{"project"}, {"project", "-h"}, {"project", "--help"}, {"help", "project"},
		{"project", "new", "--help"}, {"help", "project", "new"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := New("kit", args, WithBuiltins(), WithPath(t.TempDir()), WithStdio(strings.NewReader(""), &out, &errOut))
			app.HelpText = "Application notes."
			app.Add(Command{Name: "project", Children: []Command{
				{Name: "new", Summary: "Create a project", Usage: "[--name NAME]", Parameters: []Parameter{{Name: "name", Type: "string", Required: true, Choices: []string{"alpha", "beta"}}}, Run: func(context.Context, []string) error { t.Fatal("help executed new"); return nil }},
				{Name: "list", Summary: "List projects", Run: func(context.Context, []string) error { t.Fatal("help executed list"); return nil }},
			}}, Command{Name: "task", Children: []Command{{Name: "done", Summary: "Finish a task"}}})
			if err := app.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if errOut.Len() != 0 {
				t.Fatal(errOut.String())
			}
			for _, want := range []string{"usage: kit project", "Create a project", "Application notes."} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q in %s", want, out.String())
				}
			}
			if strings.Contains(out.String(), "Finish a task") {
				t.Fatalf("unrelated command in contextual help: %s", out.String())
			}
			if len(args) >= 3 {
				for _, want := range []string{"--name <string> (required)", "choices: alpha, beta"} {
					if !strings.Contains(out.String(), want) {
						t.Fatalf("missing parameter %q in %s", want, out.String())
					}
				}
			}
		})
	}
}

func TestCommandGroupResolutionAndMissingCommand(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
		code int
	}{
		{[]string{"project", "list"}, "projects: alpha, beta", 0},
		{[]string{"pro", "ls"}, "projects: alpha, beta", 0},
		{[]string{"project", "li"}, "projects: alpha, beta", 0},
		{[]string{"project", "n"}, "ambiguous", 1},
		{[]string{"project", "missing"}, "List projects", 1},
		{[]string{"project", "admin"}, "Configure access", 0},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := New("kit", tc.args, WithPath(t.TempDir()), WithStdio(strings.NewReader(""), &out, &errOut))
			app.Add(Command{Name: "project", Children: []Command{
				{Name: "list", Aliases: []string{"ls"}, Summary: "List projects", Handle: func(c *Ctx) error { return c.Println("projects: alpha, beta") }},
				{Name: "new", Summary: "Create a project"}, {Name: "next", Summary: "Next project"},
				{Name: "admin", Children: []Command{{Name: "access", Summary: "Configure access"}}},
			}})
			err := app.Run(context.Background())
			if ExitCode(err) != tc.code {
				t.Fatalf("exit %d: %v", ExitCode(err), err)
			}
			combined := out.String() + errOut.String()
			if !strings.Contains(combined, tc.want) {
				t.Fatalf("missing %q in %s", tc.want, combined)
			}
		})
	}
}

func TestCommandGroupKeepsParentPreparationOutOfHelp(t *testing.T) {
	var out bytes.Buffer
	app := New("kit", []string{"project"}, WithPath(t.TempDir()), WithStdio(nil, &out, &out))
	app.Default = func(context.Context, []string) error { return fmt.Errorf("unavailable daemon") }
	app.Add(Command{Name: "project", Children: []Command{{Name: "list", Summary: "List projects"}}})
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "List projects") {
		t.Fatal(out.String())
	}
}
