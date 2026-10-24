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
| `Handler`, `Ctx`, `App.Cmd`, input helpers and `Ctx.Sub` | `handler.go` |
| `Catalog`, `Entry`, `Claim`, ownership and shadowing, `Resolve` | `catalog.go` |
| PATH scan, `FS`, `Discover`, `DiscoverAll` (extra prefixes), `Executable` | `discover.go` |
| The `o` root bin (builtins + `o-<tool>`) | `cmd/o` |
| Help listing | `help.go` |
| Opt-in `help`, `commands`, `which` | `builtins.go` |
| Runnable bin-to-bin delegation examples | `examples/delegation.go` |

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
- `Cmd` registers ordinary `Command` rows. `Handle` takes precedence over
  legacy `Run`; preserve both entry points through `RunEntry`.
- Input helpers open OS files lazily; `WithFS` is discovery-only. Readers
  own opened files, never stdin. See README for line limits and input rules.
- Externals get `ODEN_PARENT` and `ODEN_ROOT`; a child bin is `<bin>-<sub>`.
- Extra prefixes are opt-in (`WithPrefixes`) and always rank after the bin's
  own `<bin>-<sub>`; the loser is shadowed, never dropped. No bare-name
  lookup: a command is always `<prefix>-<sub>`, never an arbitrary PATH tool.
- `cmd/o` is the family root: builtins and `o-<tool>` externals only; the
  TUI host is `o-nabe`/`o-tui` on PATH. No TUI code in oden.
- Never bind function keys anywhere (examples and a future TUI included).

## Checks

```sh
gofmt -l . && go vet ./... && go test -race ./...
go test -run Golden -update   # after an intended help format change
```
