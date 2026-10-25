package oden

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Help writes commands separated by audience, then by optional category,
// including their kind, summary, source and shadowed claims.
func (a *App) Help(w io.Writer) {
	fmt.Fprintf(w, "usage: %s COMMAND [ARGS...]\n\n", a.Name)
	entries := a.Catalog().Entries
	var human, machine []*Entry
	for _, e := range entries {
		if e.Audience == AudienceMachine {
			machine = append(machine, e)
		} else {
			human = append(human, e)
		}
	}
	if len(machine) == 0 {
		fmt.Fprintln(w, "commands:")
		a.List(w, human)
		return
	}
	if len(human) != 0 {
		fmt.Fprintln(w, "Human commands:")
		a.List(w, human)
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "Machine commands (JSON / JSON Lines):")
	a.List(w, machine)
}

// List writes entries grouped by optional category, with uncategorized
// entries under Other. Without categories it keeps a flat listing.
func (a *App) List(w io.Writer, entries []*Entry) {
	categorized := false
	for _, e := range entries {
		if e.Category != "" {
			categorized = true
			break
		}
	}
	if !categorized {
		a.listEntries(w, entries)
		return
	}
	groups := map[string][]*Entry{}
	for _, e := range entries {
		category := e.Category
		if category == "" {
			category = "Other"
		}
		groups[category] = append(groups[category], e)
	}
	keys := make([]string, 0, len(groups))
	for category := range groups {
		keys = append(keys, category)
	}
	sort.Strings(keys)
	for _, category := range keys {
		fmt.Fprintf(w, "\n%s:\n", category)
		a.listEntries(w, groups[category])
	}
}

func (a *App) listEntries(w io.Writer, entries []*Entry) {
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
