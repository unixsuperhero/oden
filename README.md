# oden

The lowest layer for extensible, subcommand-driven CLI tools in Go: a bin's
inline commands merged with third-party executables on PATH, git- and
hiiro-style. No dependencies outside the standard library, no application
knowledge. `github.com/unixsuperhero/oden`.

```go
app := oden.New("hello", os.Args[1:], oden.WithBuiltins())
app.Cmd("greet", "say hello", func(c *oden.Ctx) error {
	return c.Printf("hello, %s\n", c.Arg(0, "world"))
}, "hi")
os.Exit(oden.Report(os.Stderr, "hello", app.Run(context.Background())))
```

## Handlers and input

`Cmd` registers a `Handler` with arguments, context, and the app's streams.
`Command{Run: ...}` remains supported. `Command.Handle` takes precedence
when both fields are set. Registration through `Cmd` uses the same catalog,
ownership rules, aliases, and caller-source tracking as `Add`.

```go
app.Cmd("cat", "concatenate files or stdin", func(c *oden.Ctx) error {
	in := c.Input()
	defer in.Close()
	_, err := io.Copy(c.Stdout, in)
	return err
})
```

| Input | Example | Handler access |
|---|---|---|
| Positional arguments | `hello greet Ada` | `c.Args`, `c.Arg(0, "world")` |
| Pipe | `printf 'hello\n' \| hello cat` | `c.Stdin` or `c.Input()` |
| Redirection | `hello cat < notes.txt` | Same stdin stream |
| File paths | `hello cat a.txt b.txt` | `c.Input()` |
| Files mixed with stdin | `printf 'middle\n' \| hello cat a.txt - b.txt` | `-` reads stdin in order |

`Open(paths...)` opens OS files lazily, one at a time, and reports failures
through `Read`. No paths means stdin. Close the reader when done; closing
never closes stdin. `WithFS` affects executable discovery, not input files.
`Text()` reads all input into memory. `Lines()` streams newline-stripped
lines and an optional final error, closing files even on an early `break`.
It uses `bufio.Scanner`'s default 64 KiB token limit. For larger lines, use
`Open` with your own scanner or buffered reader.

Input helpers concatenate bytes without inserting separators between files.
They consume stdin from its current position; repeated `-` does not rewind it.
Arguments are paths only when you call `Input`, `Text`, or `Lines`.
Parse flags yourself and pass the remaining paths to `Open` when needed.
No `Piped()` heuristic is exposed: pipes and redirects both work by reading
stdin, and a generic injected `io.Reader` has no portable terminal status.
Context cancellation is available through `c.Context`; it does not interrupt
a blocking file or stdin read automatically.

For an in-process command group, `Sub` inherits the app's discovery settings,
environment, root, and streams, but not extra prefixes:

```go
app.Cmd("admin", "administration", func(c *oden.Ctx) error {
	return c.Sub(oden.WithBuiltins()).
		Cmd("status", "show status", func(c *oden.Ctx) error {
			return c.Println("ready")
		}).Run(c)
})
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
`Add` or `Cmd`. The PATH list and the file system are injectable (`WithPath`,
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
`commands --json`, so nothing is hidden silently.

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

**Builtins** (opt-in, `WithBuiltins()`): usage separates commands by audience.
Human-facing commands print readable text; machine-facing commands retain
their established JSON formats without requiring an output flag.

| Audience | Invocation | Output |
|---|---|---|
| Human | `help`, `-h`, `--help` | Commands grouped by audience and optional domain category, with kind, summary, source and shadowed claims |
| Human | `info [COMMAND]` | Bin information, or a command's summary, audience, category, kind, source and aliases |
| Human | `version` | Embedded module version, full commit SHA, commit time, dirty-build flag, Go version and module path |
| Human | `which WORD` | `name kind source` for the resolved command |
| Machine | `commands` | Entry array: name, kind, aliases, summary, category, audience, source, prefix, shadowed |
| Machine | `catalog` | ochazuke/cli schema 1: command noun with list, show and state; no bindings or pages |
| Machine | `command list` | Command row array |
| Machine | `command show NAME` | One command row |
| Machine | `state`, `command state` | One command row per JSON Line |

`info COMMAND`, `which WORD` and `command show NAME` resolve names, aliases
and unique prefixes. A row has `noun`, `id`, `label` and `data`; data includes
kind, path, source, aliases, shadows, prefix and optional summary, category
and audience. Build info supplies the catalog version. `WriteCatalog` and
`WriteState` remain explicit JSON APIs.

Machine commands preserve their original argument handling: `commands`,
`catalog`, `state`, `command list` and `command state` ignore trailing
arguments, including the host convention `--json '{}' --yes`. `command show`
still takes exactly one name, and `which` exactly one word. No `-j` is
required, no `-H` / `--human` mode exists, and output never changes based on
whether stdout is a terminal. Use `help` rather than changing a machine
command's output format.

`info -h` / `info --help` prints its usage; `info -- -h` describes the help
alias. Invalid info arguments exit 2; unresolved names exit 1. Existing
builtins retain their prior usage-error exit behavior.

**Version provenance.** `version` is included by `WithBuiltins()`, so `o`
and applications using the builtins acquire it when rebuilt against this
oden version. App registrations can override it like any other builtin.
The command reads Go's embedded `runtime/debug.ReadBuildInfo`, not the
current directory, repository, or network. `Modified` describes the main
module's source repository at build time, not its current status. It does
not report dirty source in locally replaced dependency repositories.

Use a normal package build with VCS stamping enabled to include the commit,
for example `go build -buildvcs=true -o ~/bin/o ./cmd/o`. The version is
the embedded main-module version, including `(devel)` or a pseudo-version
when that is how the binary was built. Builds with no VCS metadata, such as
`-buildvcs=false` or some `go run` builds, print `unknown` for the commit,
time and modified flag rather than claiming a clean or released build.
`version -h` / `version --help` prints usage; unexpected arguments exit 2.
No output-format flag is needed.

**Audience.** Set `Command.Audience` to `AudienceMachine` for a structured
machine endpoint or `AudienceHuman` for readable output. Unspecified
commands, including discovered externals, are presented as human-facing.
Audience is presentation metadata, not an output serializer or dispatch
rule. An app command overriding a builtin inherits that builtin's audience
unless it explicitly sets one. Thus an overridden runtime `catalog` stays
in the machine group without changing its handler. The app still owns the
name whenever added, and the builtin remains a shadowed claim.

**Family flag convention.** Give long flags single-letter aliases from
`a-zA-Z` unless every available letter in that command's scope is already
assigned. Reserve `-h` for help. Human commands default to readable output;
documented machine endpoints retain stable structured output. Oden does not
parse arbitrary handlers' or external processes' flags. Ochazuke's existing
`--json OBJECT` input protocol is unchanged.

**Categories.** Audience and domain category are independent. Set
`Command.Category` for inline commands, or use
`WithCategories(map[string]string{"task": "Work", "web": "Network"})` to
assign or override groups by canonical name, including discovered externals.
`Cmd` users can supply the same option. Within each audience, help sorts
category headings alphabetically, keeps commands sorted within each group,
and retains uncategorized commands under `Other`. Without categories a group
stays flat. Apps with no machine commands retain the plain `commands:`
listing. JSON entries and command rows retain optional category/audience
metadata. Neither affects resolution.

Oden does not execute discovered tools to collect metadata. External
categories must be assigned by the owning app. `Child` does not inherit
category assignments because its command names belong to a different scope.

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
the builtins and nothing else built in. Every word follows the normal rule,
so a tool joins the family by installing as `o-<tool>`; by convention the
TUI host, nabe, is installed as both `o-nabe` and `o-tui`:

| Word | Resolves to, highest first |
|---|---|
| `o help`, `o info [W]`, `o version`, `o which W`; `o commands`, `o catalog`, `o state`, `o command list\|show\|state` | inline builtins; first group human-facing, second group JSON-first |
| `o tui ARGS`, `o nabe ARGS` | external `o-tui` / `o-nabe` (nabe) |
| `o proj new x` | external `o-proj new x` (`ODEN_PARENT=o`) |
| `o pr new x` | the same, by unique prefix |

Install: `go build -o ~/bin/o ./cmd/o`. Another app shares the family's
tools with `WithPrefixes("o")`, as `mtm` does (`mtm proj` runs `o-proj`).

## One oden bin delegates to another

[`examples/delegation.go`](examples/delegation.go) contains runnable apps
selected by executable name. Real projects can keep each app in its own
package. Build the example executables into a temporary directory:

```sh
bins=$(mktemp -d)
for name in toolbox toolbox-files toolbox-files-text desk; do
	go build -o "$bins/$name" ./examples/delegation.go
done
export PATH="$bins:$PATH"
```

### A parent delegates to a child bin

`toolbox` has no inline `files` handler. Discovery finds `toolbox-files`,
which is another oden app with an inline `info` handler:

```sh
toolbox files info Ada
# bin=toolbox-files parent=toolbox root=toolbox arg=Ada
toolbox fi info Ada
# Same result through a unique prefix.
```

### Three bins form a command chain

`toolbox-files` discovers `toolbox-files-text`. Each bin consumes its own
command word and forwards the rest, together with stdin, stdout, and stderr:

```sh
printf 'hello\n' | toolbox files text show
# hello
toolbox files text show README.md
# Prints README.md.
toolbox files text show < README.md
# Prints the same input through stdin.
```

### An unrelated bin shares another family's commands

`desk` opts into `WithPrefixes("toolbox")`. It can dispatch `files` to the
same `toolbox-files` executable without a wrapper or duplicate handler:

```sh
desk files info Ada
# bin=toolbox-files parent=desk root=toolbox-files arg=Ada
printf 'shared\n' | desk files text show
# shared
```

The environment contains `ODEN_ROOT=desk` on the first shared-prefix hop.
`New` adopts that root only when the executable name starts with `desk-`.
Here it does not, so `toolbox-files` starts its own root, as the output shows.
An installed `desk-files` wins over `toolbox-files`; the catalog records the
losing executable as shadowed.

## API

The signatures below list the consumer methods. `go doc -all .` also shows
every exported data field and its documentation.

```go
type Handler func(*Ctx) error
type Ctx struct {
	context.Context
	App *App
	Name string
	Args []string
	Stdin io.Reader
	Stdout, Stderr io.Writer
}
type Option func(*App)
func New(name string, args []string, opts ...Option) *App
func WithBuiltins() Option
func WithDefault(run func(context.Context, []string) error) Option
func WithPrefixes(names ...string) Option
func WithCategories(categories map[string]string) Option
func WithPath(path string) Option
func WithFS(fsys FS) Option
func WithEnv(env []string) Option
func WithStdio(in io.Reader, out, errOut io.Writer) Option
func Child(parent *App, sub string, args []string) *App
func (a *App) Cmd(name, summary string, run Handler, aliases ...string) *App
func (a *App) Add(cmds ...Command) *App
func (a *App) Catalog() *Catalog
func (a *App) RootName() string
func (a *App) Run(ctx context.Context) error
func (a *App) RunEntry(ctx context.Context, e *Entry, args []string) error
func (a *App) Help(w io.Writer)
func (a *App) List(w io.Writer, entries []*Entry)
func (c *Ctx) Arg(i int, def string) string
func (c *Ctx) Open(paths ...string) io.ReadCloser
func (c *Ctx) Input() io.ReadCloser
func (c *Ctx) Text() (string, error)
func (c *Ctx) Lines() iter.Seq2[string, error]
func (c *Ctx) Printf(f string, a ...any) error
func (c *Ctx) Println(a ...any) error
func (c *Ctx) Usage(synopsis string) error // prints usage; exit code 2
func (c *Ctx) Sub(opts ...Option) *App
func NewCatalog(bin string, inline []*Command, externals, shadowed []External) *Catalog
func (c *Catalog) Resolve(word string) Resolution
func (c *Catalog) Lookup(token string) *Entry
func (r Resolution) Resolved() bool
func Discover(bin, path string, fsys FS) (found, shadowed []External)
func DiscoverAll(bins []string, path string, fsys FS) (found, shadowed []External)
func PathDirs(path string) []string
func Executable(fsys FS, path string) bool
func ExitCode(err error) int
func Report(w io.Writer, prefix string, err error) int
func (e *ExitError) Error() string
func (e *ExitError) Unwrap() error
```

Exported data types and values:

| Name | Consumer-facing data |
|---|---|
| `Command` | `Name`, `Aliases`, `Summary`, legacy `Run`, `Handle`, `Source` |
| `App` | `Name`, `Root`, `Args`, `Default`, `Path`, `FS`, `Env`, `Prefixes`, `Stdin`, `Stdout`, `Stderr` |
| `Catalog` | `Bin`, sorted `Entries` |
| `Entry` | `Name`, `Kind`, `Aliases`, `Summary`, `Source`, `Prefix`, `Shadowed`, `Command`, `Path` |
| `Claim` | `Name`, `Kind`, `Source`, `AliasOf` |
| `External` | `Name`, `Path`, `Prefix` |
| `Resolution` | `Query`, `Entry`, `Exact`, `Ambiguous`, `Candidates` |
| `Kind` | `KindInline`, `KindExternal` |
| `FS` | `ReadDir(string) ([]fs.DirEntry, error)`, `Stat(string) (fs.FileInfo, error)` |
| `OS` | OS-backed discovery filesystem |
| `ExitError` | `Code int`, `Err error` |
| `EnvParent`, `EnvRoot` | Environment keys `ODEN_PARENT`, `ODEN_ROOT` |

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
