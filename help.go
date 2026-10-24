package oden

import (
	"fmt"
	"io"
	"strings"
)

// Help writes the full listing: every entry, its kind, summary and source,
// and the claims it shadows.
func (a *App) Help(w io.Writer) {
	fmt.Fprintf(w, "usage: %s COMMAND [ARGS...]\n\ncommands:\n", a.Name)
	a.List(w, a.Catalog().Entries)
}

// List writes one row per entry (name and aliases, kind, summary, source)
// and a "shadows" line under it per losing claim.
func (a *App) List(w io.Writer, entries []*Entry) {
	names := make([]string, len(entries))
	nameW, sumW := 0, 0
	for i, e := range entries {
		names[i] = strings.Join(append([]string{e.Name}, e.Aliases...), ", ")
		nameW = max(nameW, len(names[i]))
		sumW = max(sumW, len(e.Summary))
	}
	for i, e := range entries {
		line := fmt.Sprintf("  %-*s  %-10s  %-*s  %s", nameW, names[i], "("+string(e.Kind)+")", sumW, e.Summary, e.Source)
		fmt.Fprintln(w, strings.TrimRight(line, " "))
		for _, c := range e.Shadowed {
			label := string(c.Kind)
			if c.AliasOf != "" {
				label = fmt.Sprintf("alias %s of %s", c.Name, c.AliasOf)
			}
			fmt.Fprintf(w, "  %-*s  shadows %s %s\n", nameW, "", label, c.Source)
		}
	}
}
