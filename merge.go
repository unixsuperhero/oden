package oden

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// probeTimeout bounds each question asked of a merged external.
var probeTimeout = 2 * time.Second

// mergeSource is the external that shares an inline command group's name
// (BIN-NAME, or PREFIX-NAME through WithPrefixes): the group resolves its
// next word over the union of its children and the external's
// subcommands.
type mergeSource struct {
	path   string // the external's executable
	name   string // its file name, the Source of its subcommand rows
	parent string // the bin whose group it joins: ODEN_PARENT when it runs

	passThrough bool // it reported no subcommands: unknown words go to it
}

// subcommand is one subcommand an external reports.
type subcommand struct {
	Name     string   `json:"name"`
	Aliases  []string `json:"aliases,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Audience Audience `json:"audience,omitempty"`
}

// subcommandCache is one external's answer, keyed by its path, size and
// modification time.
type subcommandCache struct {
	Path        string       `json:"path"`
	Size        int64        `json:"size"`
	ModTime     int64        `json:"mtime_ns"`
	From        string       `json:"from"` // catalog, command list, commands or none
	Subcommands []subcommand `json:"subcommands"`
}

// mergedClaim is the external claim a group entry merges, or nil.
func mergedClaim(e *Entry) *Claim {
	for i := range e.Shadowed {
		if e.Shadowed[i].Merged {
			return &e.Shadowed[i]
		}
	}
	return nil
}

// mergeInto adds the external's subcommands to c below every inline and
// PATH claim: a name already owned keeps it and records the external's
// subcommand as shadowed. With no subcommands the group passes unknown
// words through.
func (m *mergeSource) mergeInto(a *App, c *Catalog) {
	info, err := os.Stat(m.path)
	if err != nil {
		return // not on this file system (a WithFS fake): nothing to merge
	}
	subs := a.externalSubcommands(m, info)
	if len(subs) == 0 {
		m.passThrough = true
		return
	}
	var added []*Entry
	aliases := map[*Entry][]string{}
	for _, s := range subs {
		if s.Name == "" {
			continue
		}
		if w := c.tokens[s.Name]; w != nil {
			if w.Sub != s.Name || w.Path != m.path {
				w.Shadowed = append(w.Shadowed, Claim{Name: s.Name, Kind: KindExternal, Source: m.name + " " + s.Name})
			}
			continue
		}
		e := &Entry{Name: s.Name, Kind: KindExternal, Aliases: []string{}, Summary: s.Summary, Audience: s.Audience,
			Source: m.name, Shadowed: []Claim{}, Path: m.path, Sub: s.Name, parent: m.parent}
		c.tokens[s.Name] = e
		added = append(added, e)
		aliases[e] = s.Aliases
	}
	for _, e := range added {
		for _, alias := range aliases[e] {
			if w := c.tokens[alias]; w != nil {
				if w != e {
					w.Shadowed = append(w.Shadowed, Claim{Name: alias, Kind: KindExternal, Source: m.name + " " + e.Name, AliasOf: e.Name})
				}
				continue
			}
			e.Aliases = append(e.Aliases, alias)
			c.tokens[alias] = e
		}
	}
	c.Entries = append(c.Entries, added...)
	sort.Slice(c.Entries, func(i, j int) bool { return c.Entries[i].Name < c.Entries[j].Name })
}

// externalSubcommands is what the external reports, from the cache when
// its path, size and mtime are unchanged, else by asking it (and caching
// the answer, an empty one too).
func (a *App) externalSubcommands(m *mergeSource, info os.FileInfo) []subcommand {
	file := ""
	if dir := a.cacheDir(); dir != "" {
		sum := sha256.Sum256([]byte(m.path))
		file = filepath.Join(dir, "subcommands-"+hex.EncodeToString(sum[:8])+".json")
		var cached subcommandCache
		if data, err := os.ReadFile(file); err == nil && json.Unmarshal(data, &cached) == nil &&
			cached.Path == m.path && cached.Size == info.Size() && cached.ModTime == info.ModTime().UnixNano() {
			return cached.Subcommands
		}
	}
	subs, from := a.askSubcommands(m)
	if file != "" {
		entry := subcommandCache{Path: m.path, Size: info.Size(), ModTime: info.ModTime().UnixNano(), From: from, Subcommands: subs}
		if data, err := json.Marshal(entry); err == nil && os.MkdirAll(filepath.Dir(file), 0o755) == nil {
			if tmp, err := os.CreateTemp(filepath.Dir(file), ".subcommands-*"); err == nil {
				_, werr := tmp.Write(data)
				if tmp.Close() != nil || werr != nil || os.Rename(tmp.Name(), file) != nil {
					os.Remove(tmp.Name())
				}
			}
		}
	}
	return subs
}

// cacheDir is $XDG_CACHE_HOME/oden, else ~/.cache/oden.
func (a *App) cacheDir() string {
	if dir := a.getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "oden")
	}
	home := a.getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".cache", "oden")
}

// askSubcommands asks the external for its catalog (an ochazuke catalog's
// nouns, or a plain oden catalog's command rows), then for commands --json,
// and reports which answer it used ("none" when nothing was usable).
func (a *App) askSubcommands(m *mergeSource) ([]subcommand, string) {
	var doc struct {
		Schema int `json:"schema"`
		Nouns  []struct {
			Name  string `json:"name"`
			Verbs []struct {
				Name string `json:"name"`
			} `json:"verbs"`
		} `json:"nouns"`
	}
	if data := a.ask(m, "catalog"); data != nil && json.Unmarshal(data, &doc) == nil && doc.Schema >= 1 && len(doc.Nouns) != 0 {
		if len(doc.Nouns) == 1 && doc.Nouns[0].Name == CommandNoun {
			var rows []struct {
				ID   string `json:"id"`
				Data struct {
					Summary  string   `json:"summary"`
					Aliases  []string `json:"aliases"`
					Audience Audience `json:"audience"`
				} `json:"data"`
			}
			if data := a.ask(m, CommandNoun, "list"); data != nil && json.Unmarshal(data, &rows) == nil && len(rows) != 0 {
				subs := make([]subcommand, 0, len(rows))
				for _, r := range rows {
					subs = append(subs, subcommand{Name: r.ID, Aliases: r.Data.Aliases, Summary: r.Data.Summary, Audience: r.Data.Audience})
				}
				return subs, "command list"
			}
		} else {
			subs := make([]subcommand, 0, len(doc.Nouns))
			for _, n := range doc.Nouns {
				verbs := make([]string, 0, len(n.Verbs))
				for _, v := range n.Verbs {
					verbs = append(verbs, v.Name)
				}
				subs = append(subs, subcommand{Name: n.Name, Summary: strings.Join(verbs, " ")})
			}
			return subs, "catalog"
		}
	}
	var entries []subcommand
	if data := a.ask(m, "commands", "--json"); data != nil && json.Unmarshal(data, &entries) == nil && len(entries) != 0 {
		return entries, "commands"
	}
	return nil, "none"
}

// ask runs the external with args under probeTimeout, with no stdin and
// the environment an external runs with, and returns its stdout when it
// exits 0.
func (a *App) ask(m *mergeSource, args ...string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.path, args...)
	cmd.Env = a.externalEnv(m.parent)
	cmd.Stderr = io.Discard
	cmd.WaitDelay = probeTimeout / 4
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}
