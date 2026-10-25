package oden

import (
	"bytes"
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// catalogScript is an ochazuke-style external: it answers catalog with
// nouns deploy, list and lint, and echoes any other invocation.
const catalogScript = `#!/bin/sh
echo "$*" >> "$LOG"
case "$1" in
catalog) printf '%s\n' '{"schema":1,"name":"kit-project","nouns":[{"name":"deploy","verbs":[{"name":"run"},{"name":"undo"}]},{"name":"list","verbs":[]},{"name":"lint","verbs":[]}]}' ;;
*) echo "ext: $* parent=$ODEN_PARENT root=$ODEN_ROOT" ;;
esac
`

// plainScript answers nothing usable, like an ordinary shell script.
const plainScript = `#!/bin/sh
echo "$*" >> "$LOG"
echo "script: $*"
`

// commandsScript answers only commands --json.
const commandsScript = `#!/bin/sh
echo "$*" >> "$LOG"
case "$1" in
commands) printf '%s\n' '[{"name":"sync","aliases":["sy"],"summary":"sync it"}]' ;;
catalog) exit 2 ;;
*) echo "cmds: $*" ;;
esac
`

type mergeEnv struct {
	dir, log string
	out, err bytes.Buffer
}

func newMergeEnv(t *testing.T, scripts map[string]string) *mergeEnv {
	t.Helper()
	m := &mergeEnv{dir: t.TempDir()}
	m.log = filepath.Join(t.TempDir(), "log")
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(m.dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

// app is a kit bin with a project group (new, list) and a tool group
// (new), over m's PATH and a fresh cache directory per env.
func (m *mergeEnv) app(t *testing.T, args []string, opts ...Option) *App {
	t.Helper()
	m.out.Reset()
	m.err.Reset()
	env := []string{"LOG=" + m.log, "XDG_CACHE_HOME=" + filepath.Join(filepath.Dir(m.log), "cache"), "PATH=/usr/bin:/bin"}
	opts = append([]Option{WithBuiltins(), WithPath(m.dir), WithEnv(env), WithStdio(strings.NewReader(""), &m.out, &m.err)}, opts...)
	app := New("kit", args, opts...)
	say := func(word string) func(*Ctx) error {
		return func(c *Ctx) error { return c.Println("inline", word, strings.Join(c.Args, " ")) }
	}
	app.Add(Command{Name: "project", Children: []Command{
		{Name: "new", Summary: "create a project", Handle: say("new")},
		{Name: "list", Summary: "list projects", Handle: say("list")},
	}}, Command{Name: "tool", Children: []Command{
		{Name: "new", Summary: "create a tool", Handle: say("tool new")},
	}})
	return app
}

func (m *mergeEnv) run(t *testing.T, args ...string) error {
	t.Helper()
	return m.app(t, args).Run(context.Background())
}

// calls are the external's invocations so far, one per line.
func (m *mergeEnv) calls() []string {
	data, _ := os.ReadFile(m.log)
	return strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(data)), " ", "_"))
}

func TestMergeInlineOnlyVerbDoesNotAskTheExternal(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-project": catalogScript})
	if err := m.run(t, "project", "new", "x"); err != nil || m.out.String() != "inline new x\n" {
		t.Fatalf("%v %q %s", err, m.out.String(), m.err.String())
	}
	if calls := m.calls(); len(calls) != 0 {
		t.Fatalf("external asked for an exact inline word: %v", calls)
	}
}

func TestMergeExternalOnlyVerbExecsTheExternal(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-project": catalogScript})
	if err := m.run(t, "project", "deploy", "a", "b"); err != nil || m.out.String() != "ext: deploy a b parent=kit root=kit\n" {
		t.Fatalf("%v %q %s", err, m.out.String(), m.err.String())
	}
	if err := m.run(t, "pro", "dep", "c"); err != nil || m.out.String() != "ext: deploy c parent=kit root=kit\n" {
		t.Fatalf("prefix: %v %q %s", err, m.out.String(), m.err.String())
	}
}

func TestMergeCollisionRunsInlineAndHelpShowsShadowed(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-project": catalogScript})
	if err := m.run(t, "project", "list"); err != nil || m.out.String() != "inline list \n" {
		t.Fatalf("%v %q", err, m.out.String())
	}
	if err := m.run(t, "project", "--help"); err != nil {
		t.Fatal(err)
	}
	help := m.out.String()
	for _, want := range []string{
		"usage: kit project COMMAND",
		"list    (inline)    list projects",
		"shadows external kit-project list",
		"deploy  (external)  run undo",
		"kit-project\n",
		"lint    (external)",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("missing %q in\n%s", want, help)
		}
	}
	if err := m.run(t, "help"); err != nil || !strings.Contains(m.out.String(), "merges external "+filepath.Join(m.dir, "kit-project")) {
		t.Fatalf("root help: %v\n%s", err, m.out.String())
	}
	direct := runDirect(t, filepath.Join(m.dir, "kit-project"), "list")
	if direct != "ext: list parent= root=\n" {
		t.Fatalf("direct = %q", direct)
	}
}

func TestMergeCrossSideAmbiguousPrefix(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-project": catalogScript})
	err := m.run(t, "project", "li")
	e := m.err.String()
	if ExitCode(err) != 1 || !strings.Contains(e, `"li" is ambiguous`) ||
		!strings.Contains(e, "list  (inline)") || !strings.Contains(e, "lint  (external)") || !strings.Contains(e, "kit-project") {
		t.Fatalf("%v\n%s", err, e)
	}
}

func TestMergeNotFoundSuggestsBothSides(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-project": catalogScript})
	err := m.run(t, "project", "lit")
	e := m.err.String()
	if ExitCode(err) != 1 || !strings.HasPrefix(e, "kit project: subcommand \"lit\" not found\ndid you mean: lint, list\n") || !strings.Contains(e, "deploy") {
		t.Fatalf("%v\n%s", err, e)
	}
}

func TestMergePassThroughForAnExternalWithoutACatalog(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-tool": plainScript})
	if err := m.run(t, "tool", "anything", "a"); err != nil || m.out.String() != "script: anything a\n" {
		t.Fatalf("%v %q %s", err, m.out.String(), m.err.String())
	}
	if err := m.run(t, "tool", "new"); err != nil || m.out.String() != "inline tool new \n" {
		t.Fatalf("inline: %v %q", err, m.out.String())
	}
	if err := m.run(t, "tool"); err != nil || !strings.Contains(m.out.String(), "Other words run kit-tool ("+filepath.Join(m.dir, "kit-tool")+").") {
		t.Fatalf("help: %v\n%s", err, m.out.String())
	}
	// catalog and commands --json were asked once, then cached as none.
	if calls := m.calls(); strings.Join(calls, ",") != "catalog,commands_--json,anything_a" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestMergeCommandsJSONAndPrefixFamily(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"fam-project": commandsScript})
	app := m.app(t, []string{"project", "sy", "x"}, WithPrefixes("fam"))
	if err := app.Run(context.Background()); err != nil || m.out.String() != "cmds: sync x\n" {
		t.Fatalf("%v %q %s", err, m.out.String(), m.err.String())
	}
	app = m.app(t, []string{"project"}, WithPrefixes("fam"))
	if err := app.Run(context.Background()); err != nil || !strings.Contains(m.out.String(), "sync, sy  (external)  sync it") {
		t.Fatalf("%v\n%s", err, m.out.String())
	}
}

func TestMergeCacheHitAndMtimeInvalidation(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-project": catalogScript})
	for range 2 {
		if err := m.run(t, "project", "deploy"); err != nil {
			t.Fatal(err)
		}
	}
	if calls := m.calls(); strings.Join(calls, ",") != "catalog,deploy,deploy" {
		t.Fatalf("cache missed: %v", calls)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(m.dir, "kit-project"), later, later); err != nil {
		t.Fatal(err)
	}
	if err := m.run(t, "project", "deploy"); err != nil {
		t.Fatal(err)
	}
	if calls := m.calls(); strings.Join(calls, ",") != "catalog,deploy,deploy,catalog,deploy" {
		t.Fatalf("mtime change not seen: %v", calls)
	}
}

func TestNoExternalExecWithoutASameNamedExternal(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-other": plainScript})
	for _, args := range [][]string{{"project", "new"}, {"project"}, {"project", "zz"}, {"help"}, {"pro", "li"}} {
		_ = m.run(t, args...)
	}
	if calls := m.calls(); len(calls) != 0 {
		t.Fatalf("an unrelated external ran: %v", calls)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(m.log), "cache")); !os.IsNotExist(err) {
		t.Fatalf("cache written without a merged external: %v", err)
	}
}

func TestNonGroupInlineStillShadowsTheExternal(t *testing.T) {
	m := newMergeEnv(t, map[string]string{"kit-leaf": catalogScript})
	app := m.app(t, []string{"leaf", "deploy"})
	app.Cmd("leaf", "a leaf", func(c *Ctx) error { return c.Println("leaf", strings.Join(c.Args, " ")) })
	if err := app.Run(context.Background()); err != nil || m.out.String() != "leaf deploy\n" {
		t.Fatalf("%v %q", err, m.out.String())
	}
	m.out.Reset()
	app.Help(&m.out)
	if !strings.Contains(m.out.String(), "shadows external "+filepath.Join(m.dir, "kit-leaf")) || len(m.calls()) != 0 {
		t.Fatalf("%s %v", m.out.String(), m.calls())
	}
}

func runDirect(t *testing.T, path string, args ...string) string {
	t.Helper()
	cmd := osexec.Command(path, args...)
	cmd.Env = []string{"LOG=" + filepath.Join(t.TempDir(), "log")}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
