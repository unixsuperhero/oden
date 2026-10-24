package oden

import (
	"context"
	"encoding/json"
	"fmt"
)

// WithBuiltins adds the inline commands help (also -h, --help), commands
// (the catalog as JSON) and which (where a word resolves).
func WithBuiltins() Option {
	return func(a *App) {
		a.add(1, builtins(a)...)
	}
}

func builtins(a *App) []Command {
	return []Command{
		{Name: "help", Aliases: []string{"-h", "--help"}, Summary: "list the commands", Run: func(context.Context, []string) error {
			a.Help(a.Stdout)
			return nil
		}},
		{Name: "commands", Summary: "print the command catalog as JSON", Run: func(context.Context, []string) error {
			data, err := json.MarshalIndent(a.Catalog().Entries, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(a.Stdout, "%s\n", data)
			return err
		}},
		{Name: "which", Summary: "print what a command word resolves to", Run: func(_ context.Context, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: %s which COMMAND", a.Name)
			}
			res := a.Catalog().Resolve(args[0])
			switch {
			case res.Ambiguous:
				fmt.Fprintf(a.Stderr, "%s: %q is ambiguous; did you mean one of these?\n", a.Name, res.Query)
				a.List(a.Stderr, res.Candidates)
				return &ExitError{Code: 1}
			case !res.Resolved():
				return &ExitError{Code: 1, Err: fmt.Errorf("%s: no command %q", a.Name, res.Query)}
			}
			e := res.Entry
			_, err := fmt.Fprintf(a.Stdout, "%s %s %s\n", e.Name, e.Kind, e.Source)
			return err
		}},
	}
}
