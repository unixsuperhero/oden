package oden

import (
	"context"
	"encoding/json"
	"fmt"
)

// WithBuiltins adds the inline commands help (also -h, --help), commands
// (the catalog as JSON), which (where a word resolves), catalog (the
// catalog answer, CatalogDoc), state (the command rows as JSON Lines) and
// command list|show|state (the catalog's one noun). A builtin is a
// default: a command the app registers under the same name owns it
// (cli.Mount's richer catalog, o-inventory's state), whenever it is added,
// and the builtin is listed as the claim it shadows.
func WithBuiltins() Option {
	return func(a *App) {
		cmds := builtins(a)
		for i := range cmds {
			cmds[i].builtin = true
		}
		a.add(1, cmds...)
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
		{Name: "catalog", Summary: "print the catalog answer (schema 1: the command noun) as JSON", Run: func(context.Context, []string) error {
			return a.WriteCatalog(a.Stdout)
		}},
		{Name: "state", Summary: "print every command row as JSON Lines (command state)", Run: func(context.Context, []string) error {
			return a.WriteState(a.Stdout)
		}},
		{Name: CommandNoun, Summary: "list show state", Run: func(_ context.Context, args []string) error {
			usage := fmt.Errorf("usage: %s %s list|show NAME|state", a.Name, CommandNoun)
			if len(args) == 0 {
				return usage
			}
			switch args[0] {
			case "list":
				return a.writeList(a.Stdout)
			case "state":
				return a.WriteState(a.Stdout)
			case "show":
				if len(args) != 2 {
					return fmt.Errorf("usage: %s %s show NAME", a.Name, CommandNoun)
				}
				res := a.Catalog().Resolve(args[1])
				if !res.Resolved() {
					return &ExitError{Code: 1, Err: fmt.Errorf("%s: no command %q (%d match)", a.Name, res.Query, len(res.Candidates))}
				}
				data, err := json.MarshalIndent(commandRow(res.Entry), "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(a.Stdout, "%s\n", data)
				return err
			}
			return usage
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
