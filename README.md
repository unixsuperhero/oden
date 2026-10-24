# oden

The lowest layer for extensible, subcommand-driven CLI tools in Go: a bin's
inline commands merged with third-party executables on PATH, git- and
hiiro-style. No dependencies outside the standard library, no application
knowledge. `github.com/unixsuperhero/oden`.

```go
app := oden.New("mtm", os.Args[1:], oden.WithBuiltins(), oden.WithDefault(shell))
app.Add(oden.Command{Name: "task", Aliases: []string{"t"}, Summary: "tasks", Run: runTask})
os.Exit(oden.Report(os.Stderr, "mtm", app.Run(ctx)))
```

## The contract

**Naming.** Any executable named `<bin>-<sub>` on PATH is the command `sub`
of `bin`: put `mtm-web` on PATH and `mtm web ARGS...` runs it, as `git-web`
makes `git web`. Nothing in the bin changes.

**Discovery.** PATH is read in order (an empty entry is `.`, a repeated
directory is read once at its first place). A candidate is a regular file,
or a link to one, with an executable bit; directories and non-executables
are skipped. The first `<bin>-<sub>` on PATH wins; later ones are recorded as
shadowed. Discovery runs once, on first use, and is cached until the next
`Add`. The PATH list and the file system are injectable (`WithPath`,
`WithFS`).

**Extra prefixes** (opt-in, `WithPrefixes("o")`): more bin names whose
`<name>-<sub>` executables are also this bin's commands, discovered after
the bin's own `<bin>-<sub>` (every PATH directory for the bin's prefix, then
every directory per extra prefix, in the given order). So `mtm` built with
`WithPrefixes("o")` runs `o-proj` for `mtm proj`, and an `mtm-proj` anywhere
on PATH wins and lists `o-proj` as shadowed. Entries carry their `prefix`
(JSON) and help shows the path. The descendant rule below is unchanged
across prefixes; a `Child` does not inherit extra prefixes.

**Ownership** of a token, highest first:

1. an inline command's name (the first registration wins)
2. an external's name (the first on PATH wins)
3. an inline alias (the first registration wins)

Every losing claim stays on the winner as a `shadows ...` line in help and in
`commands` JSON, so nothing is hidden silently.

**Resolution** of the first word: an exact name or alias, else the one
command with a name or alias starting with the word (`mtm ta` is `task` when
nothing else starts with `ta`). Descendants never make a prefix ambiguous:
a candidate named `<other>-...` where `<other>` is also a candidate is that
candidate's child and is dropped, so when every candidate equals the
shortest one or starts with `<shortest>-`, the shortest wins (`horror bio`
is `biographies` even though `horror-biographies-freddie-kruger` is also
on PATH and listed as `biographies-freddie-kruger`, which, as with git, can
be run directly). Any other set of several candidates is ambiguous
(`horror b` with `biographies` and `books`). The rule is the same for inline
and external commands, and is hiiro's
(`~/proj/hiiro/lib/hiiro/catalog.rb:56-57`: prefix matches are collected by
`token.start_with?(word)`, then a match whose name starts with
`"<other match>-"` is rejected).

**Running.** An inline command gets the remaining words. An external is
executed with the remaining words, the App's stdio, and the environment plus
`ODEN_PARENT=<bin>` and `ODEN_ROOT=<outermost bin>`. No word, or a word that
names nothing, runs the `Default` handler with every argument; without one,
help goes to stderr and the exit is 1. An ambiguous word prints the
candidates to stderr and exits 1, even with a `Default`.

**Nesting.** `oden.Child(app, "ai", args)` is the App named `mtm-ai`, so it
resolves `mtm-ai-skills`; an external `mtm-ai` that is itself an oden app
gets its root from `ODEN_ROOT`. `mtm ai skills` → `mtm-ai skills` →
`mtm-ai-skills`; `horror bio fre` runs `horror-biographies fre`, which (an
oden app named `horror-biographies`) runs `horror-biographies-freddie-kruger`.

**Exit codes.** `Run` returns an error; `ExitCode(err)` is 0 for nil, the
code of an `*ExitError` (an external's own exit status, 1 for help and
ambiguity), else 1. `Report(w, prefix, err)` prints the error unless it is
an `ExitError` with no cause, and returns the code.

**Builtins** (opt-in, `WithBuiltins()`): `help` (also `-h`, `--help`) lists
every command with its kind, summary, source (file:line of the `Add` call for
inline, the path for external) and shadowed claims; `commands` prints the
catalog as JSON (`name`, `kind`, `aliases`, `summary`, `source`,
`shadowed`); `which WORD` prints `name kind source` for what the word
resolves to.

```
usage: mtm COMMAND [ARGS...]

commands:
  task, t  (inline)    tasks     app.go:10
           shadows external /bin/mtm-task
  web      (external)            /bin/mtm-web
           shadows external /opt/mtm-web
```

## The `o` root

`cmd/o` is the family root bin (oden, ochazuke): an oden app named `o` with
the builtins and one inline command, `tui`, which execs `nabe` (looked up on
PATH, a clear error if absent) with the remaining words. Every other word
follows the normal rule, so a tool joins the family by installing as
`o-<tool>`:

| Word | Resolves to, highest first |
|---|---|
| `o tui ARGS` | inline `tui`: exec `nabe ARGS` |
| `o help`, `o commands`, `o which W` | inline builtins |
| `o proj new x` | external `o-proj new x` (`ODEN_PARENT=o`) |
| `o pr new x` | the same, by unique prefix |

Install: `go build -o ~/bin/o ./cmd/o`. Another app shares the family's
tools with `WithPrefixes("o")`, as `mtm` does (`mtm proj` runs `o-proj`).

## API

```go
type Command struct{ Name string; Aliases []string; Summary string; Run func(ctx, args) error; Source string }
type App struct{ Name, Root string; Args []string; Default func(ctx, args) error; Path func() string; FS FS; Env []string; Stdin; Stdout, Stderr }
func New(name string, args []string, opts ...Option) *App
func WithBuiltins() / WithDefault(fn) / WithPrefixes(names...) / WithPath(p) / WithFS(fs) / WithEnv(env) / WithStdio(in, out, err) Option
func Child(parent *App, sub string, args []string) *App
func (a *App) Add(cmds ...Command) *App
func (a *App) Catalog() *Catalog
func (a *App) Run(ctx) error
func (a *App) RunEntry(ctx, e *Entry, args) error
func (a *App) Help(w) / List(w, entries)
func NewCatalog(bin string, inline []*Command, externals, shadowed []External) *Catalog
func (c *Catalog) Resolve(word string) Resolution // Entry, Exact, Ambiguous, Candidates
func (c *Catalog) Lookup(token string) *Entry
func Discover(bin, path string, fsys FS) (found, shadowed []External)
func DiscoverAll(bins []string, path string, fsys FS) (found, shadowed []External)
func ExitCode(err error) int; func Report(w, prefix string, err error) int
```

## TUI mode and ochazuke

Not built yet; this is the intended seam. oden stays the outer shell and
knows no TUI: an oden app hands its `Catalog` (plain data: entries with
name, kind, aliases, summary, source and shadowed claims) to an
ochazuke-based TUI that defines a noun `command` whose list rows are the
catalog entries, with verbs such as `run` (calls `RunEntry`, an external
under a terminal instance) and `which`, so the TUI is one more consumer of
the same table that help and `commands` read. In the other direction an
ochazuke app mounts oden as its outer shell, as matomeru's `mtm` does: oden
resolves the first word, ochazuke's nouns are registered as inline commands
that delegate to the app's NOUN VERB shell, and `Default` runs the TUI when
no word is given.

## Checks

```sh
gofmt -l . && go vet ./... && go test -race ./...
go run ./examples/hello help
go run ./cmd/o help
```
