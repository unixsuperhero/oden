# oden: agent guide

The lowest-level layer for subcommand-driven CLIs: inline commands plus
`<bin>-<sub>` executables on PATH, resolved by name, alias or unique prefix.
Standard library only. Single flat package at the repo root, `examples/hello`
as the reference consumer, `cmd/o` the family root bin. README.md holds the contract (naming, discovery,
ownership, resolution, exit codes, env vars); keep it in sync with the code.

## Where does X live

| Concern | File |
|---|---|
| `App`, options, `Child`, `Run`, external exec, `ExitError`/`ExitCode`/`Report` | `oden.go` |
| `Catalog`, `Entry`, `Claim`, ownership and shadowing, `Resolve` | `catalog.go` |
| PATH scan, `FS`, `Discover`, `DiscoverAll` (extra prefixes), `Executable` | `discover.go` |
| The `o` root bin (`tui` execs nabe) | `cmd/o` |
| Help listing | `help.go` |
| Opt-in `help`, `commands`, `which` | `builtins.go` |

## What oden owns

Discovery, ownership, resolution, dispatch and the help/catalog views of one
bin. It does not parse flags, has no TUI, and knows no application; global
flags belong to the app before it builds the `App`, and everything after the
command word is the command's.

## Invariants

- No app knowledge. No command name appears in the core except the opt-in
  builtins, which are ordinary `Command` rows added through `Add`.
- Every command is a row: an inline `Command` or a discovered `External`,
  merged into one `Catalog`. Help, `commands`, `which` and `Run` all read that
  one table; nothing re-derives it.
- Resolution rules are data, in one place (`Catalog.Resolve` and
  `NewCatalog`): exact name or alias, then unique prefix, outer beats nested;
  ownership inline name > external > inline alias; first registration and
  first on PATH win. Never special-case a name to change the order.
- Nothing is shadowed silently: every losing claim is recorded on the winner.
- Discovery and the environment are injectable (`WithPath`, `WithFS`,
  `WithEnv`, `WithStdio`); tests never depend on the developer's PATH.
- Externals get `ODEN_PARENT` and `ODEN_ROOT`; a child bin is `<bin>-<sub>`.
- Extra prefixes are opt-in (`WithPrefixes`) and always rank after the bin's
  own `<bin>-<sub>`; the loser is shadowed, never dropped. No bare-name
  lookup: a command is always `<prefix>-<sub>`, never an arbitrary PATH tool.
- `cmd/o` is the family root: builtins, `tui` (execs nabe) and `o-<tool>`
  externals. No TUI code in oden.
- Never bind function keys anywhere (examples and a future TUI included).

## Checks

```sh
gofmt -l . && go vet ./... && go test -race ./...
go test -run Golden -update   # after an intended help format change
```
