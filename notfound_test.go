package oden

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// A word with suggestions prints them and a pointer to --help, not the
// whole usage; one with none prints the level's help.
func TestNotFoundSuggestsAndPrintsUsage(t *testing.T) {
	app, out, errOut := testApp(t, fakeFS{}, "")
	app.Add(Command{Name: "task", Summary: "work items", Run: noop},
		Command{Name: "tags", Run: noop},
		Command{Name: "project", Aliases: []string{"proj"}, Run: noop})
	for _, tc := range []struct {
		word, suggest string
	}{
		{"tasks", "did you mean: task\n"},      // a command is a prefix of the word
		{"projx", "did you mean: project\n"},   // an alias is a prefix of the word
		{"tsak", "did you mean: task\n"},       // transposition
		{"projetc", "did you mean: project\n"}, // two edits on a longer word
		{"zzzzz", ""},                          // nothing close
	} {
		errOut.Reset()
		app.Args = []string{tc.word, "x"}
		err := app.Run(context.Background())
		if ExitCode(err) != 1 || out.Len() != 0 {
			t.Fatalf("%s: exit %d, stdout %q", tc.word, ExitCode(err), out)
		}
		// A suggestion and a pointer to --help, else the level's help.
		want := `mtm: command "` + tc.word + `" not found` + "\n" + tc.suggest + "(mtm --help lists every command)\n"
		if tc.suggest == "" {
			want = `mtm: command "` + tc.word + `" not found` + "\n\nusage: mtm COMMAND [ARGS...]\n"
		}
		if !strings.HasPrefix(errOut.String(), want) || strings.Contains(errOut.String(), "work items") == (tc.suggest != "") {
			t.Fatalf("%s:\n%s\nwant prefix:\n%s", tc.word, errOut, want)
		}
	}
}

func TestNotFoundDoesNotResolveSuggestions(t *testing.T) {
	app, _, _ := testApp(t, fakeFS{}, "")
	ran := false
	app.Add(Command{Name: "task", Run: func(context.Context, []string) error { ran = true; return nil }})
	app.Args = []string{"tasks"}
	if err := app.Run(context.Background()); ExitCode(err) != 1 || ran {
		t.Fatalf("tasks ran task: %v", err)
	}
}

func TestDefaultDefersWithErrNotFound(t *testing.T) {
	app, _, errOut := testApp(t, fakeFS{}, "")
	app.Add(Command{Name: "task", Run: noop})
	var got []string
	app.Default = func(_ context.Context, args []string) error {
		got = args
		if len(args) == 0 {
			return nil
		}
		return ErrNotFound
	}
	app.Args = nil
	if err := app.Run(context.Background()); err != nil {
		t.Fatalf("no words: %v", err)
	}
	app.Args = []string{"tasks", "list"}
	err := app.Run(context.Background())
	if ExitCode(err) != 1 || !reflect.DeepEqual(got, []string{"tasks", "list"}) ||
		!strings.HasPrefix(errOut.String(), "mtm: command \"tasks\" not found\ndid you mean: task\n") {
		t.Fatalf("deferred: %v %v\n%s", err, got, errOut)
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Err != nil {
		t.Fatalf("want a silent ExitError, got %#v", err)
	}
}

func TestGroupSubcommandNotFound(t *testing.T) {
	app, out, errOut := testApp(t, fakeFS{}, "")
	app.Default = func(context.Context, []string) error { return ErrNotFound }
	app.Add(Command{Name: "task", Children: []Command{
		{Name: "list", Summary: "list tasks", Run: noop},
		{Name: "launch", Summary: "launch a task", Run: noop},
	}})
	app.Args = []string{"task", "frob"}
	err := app.Run(context.Background())
	e := errOut.String()
	if ExitCode(err) != 1 || out.Len() != 0 || !strings.HasPrefix(e, "mtm task: subcommand \"frob\" not found\n\nusage: mtm task COMMAND") ||
		!strings.Contains(e, "list tasks") || !strings.Contains(e, "launch a task") {
		t.Fatalf("%v\n%s", err, e)
	}
	errOut.Reset()
	app.Args = []string{"task", "lists"}
	if err := app.Run(context.Background()); ExitCode(err) != 1 || !strings.Contains(errOut.String(), "did you mean: list\n") {
		t.Fatalf("%v\n%s", err, errOut)
	}
}

func TestEditDistance(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		d    int
	}{{"", "", 0}, {"task", "task", 0}, {"tsak", "task", 1}, {"tasks", "task", 1}, {"kitten", "sitting", 3}} {
		if got := editDistance(tc.a, tc.b); got != tc.d {
			t.Errorf("%q %q: %d want %d", tc.a, tc.b, got, tc.d)
		}
	}
}
