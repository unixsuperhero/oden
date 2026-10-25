package oden

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is what a Default returns to defer an unknown command word
// to Run: Run then reports it as NotFound does. A Default that returns it
// with no words gets it back unchanged.
var ErrNotFound = errors.New("command not found")

// NotFound reports word as naming no command of this level on Stderr: a
// "BIN: command "WORD" not found" line ("BIN GROUP: subcommand ..." below
// the root), a "did you mean:" line when Suggest finds any, then the level's
// help. It returns an ExitError with code 1 and no cause.
func (a *App) NotFound(word string) error {
	kind := "command"
	if a.invocation != "" || a.RootName() != a.Name {
		kind = "subcommand"
	}
	fmt.Fprintf(a.Stderr, "%s: %s %q not found\n", a.helpName(), kind, word)
	if s := a.Suggest(word); len(s) != 0 {
		fmt.Fprintf(a.Stderr, "did you mean: %s\n", strings.Join(s, ", "))
	}
	fmt.Fprintln(a.Stderr)
	a.Help(a.Stderr)
	return &ExitError{Code: 1}
}

// maxSuggestions caps the "did you mean" line.
const maxSuggestions = 3

// Suggest lists the commands word may have meant, at most three, for a word
// that names none: first those whose name or alias is a prefix of word
// (tasks → task), then those within a small edit distance of word (one edit
// for a word of up to five letters, else two). It only suggests; Resolve
// never takes these.
func (a *App) Suggest(word string) []string {
	if word == "" {
		return nil
	}
	var prefixed, close []string
	limit := 1
	if len(word) > 5 {
		limit = 2
	}
	for _, e := range a.Catalog().Entries {
		tokens := append([]string{e.Name}, e.Aliases...)
		isPrefix, best := false, limit+1
		for _, t := range tokens {
			if strings.HasPrefix(t, "-") {
				continue
			}
			if strings.HasPrefix(word, t) {
				isPrefix = true
			}
			best = min(best, editDistance(word, t))
		}
		switch {
		case isPrefix:
			prefixed = append(prefixed, e.Name)
		case best <= limit:
			close = append(close, e.Name)
		}
	}
	out := append(prefixed, close...)
	if len(out) > maxSuggestions {
		out = out[:maxSuggestions]
	}
	return out
}

// editDistance is the optimal string alignment distance between a and b:
// insertions, deletions, substitutions and adjacent transpositions.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ra)][len(rb)]
}
