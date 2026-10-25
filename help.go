package oden

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Parameter describes a command argument for help. Oden does not parse it.
type Parameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Choices     []string
	Source      string
	Free        bool
}

func helpFlag(word string) bool { return word == "-h" || word == "--help" }

func (a *App) helpName() string {
	if a.invocation != "" {
		return a.invocation
	}
	return a.Name
}

func (a *App) helpNotes(w io.Writer) {
	if a.HelpText != "" {
		fmt.Fprintf(w, "\n%s\n", a.HelpText)
	}
}

func (a *App) commandHelp(w io.Writer, c *Command) error {
	var out strings.Builder
	usage := c.Usage
	if usage == "" {
		usage = "[ARGS...]"
	}
	fmt.Fprintf(&out, "usage: %s %s %s\n", a.helpName(), c.Name, usage)
	if c.Summary != "" {
		fmt.Fprintf(&out, "\n%s\n", c.Summary)
	}
	if c.Details != "" {
		fmt.Fprintf(&out, "\n%s\n", c.Details)
	}
	writeParameters(&out, c.Parameters, "  ")
	a.helpNotes(&out)
	_, err := io.WriteString(w, out.String())
	return err
}
func writeParameters(w io.Writer, parameters []Parameter, indent string) {
	for _, p := range parameters {
		required := "optional"
		if p.Required {
			required = "required"
		}
		fmt.Fprintf(w, "\n%s--%s <%s> (%s)", indent, p.Name, p.Type, required)
		if p.Description != "" {
			fmt.Fprintf(w, ": %s", p.Description)
		}
		fmt.Fprintln(w)
		if len(p.Choices) != 0 {
			fmt.Fprintf(w, "%s  choices: %s\n", indent, strings.Join(p.Choices, ", "))
		}
		if p.Source != "" {
			fmt.Fprintf(w, "%s  choices from: %s\n", indent, p.Source)
		}
		if p.Free {
			fmt.Fprintf(w, "%s  other values are accepted\n", indent)
		}
	}
}

// Help writes commands separated by audience, then by optional category,
// including their kind, summary, source and shadowed claims.
func (a *App) Help(w io.Writer) {
	defer func() {
		if a.merge != nil && a.merge.passThrough {
			fmt.Fprintf(w, "\nOther words run %s (%s).\n", a.merge.name, a.merge.path)
		}
		hasGroups := a.invocation != ""
		for _, e := range a.Catalog().Entries {
			hasGroups = hasGroups || e.Command != nil && e.Command.Children != nil
		}
		if hasGroups {
			fmt.Fprintf(w, "\nRun '%s COMMAND --help' for command details.\n", a.helpName())
		}
		a.helpNotes(w)
	}()
	fmt.Fprintf(w, "usage: %s COMMAND [ARGS...]\n\n", a.helpName())
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
		if a.invocation != "" && e.Command != nil {
			for line := range strings.Lines(e.Command.Details) {
				fmt.Fprintf(w, "    %s\n", strings.TrimSuffix(line, "\n"))
			}
			writeParameters(w, e.Command.Parameters, "    ")
		}
		for _, c := range e.Shadowed {
			label := string(c.Kind)
			if c.AliasOf != "" {
				label = fmt.Sprintf("alias %s of %s", c.Name, c.AliasOf)
			}
			verb := "shadows"
			if c.Merged {
				verb = "merges"
			}
			fmt.Fprintf(w, "  %-*s  %s %s %s\n", nameW, "", verb, label, c.Source)
		}
	}
}
