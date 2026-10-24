package oden

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata golden files")

// fakeFS is a PATH of directories holding files with modes.
type fakeFS map[string]map[string]fs.FileMode

type fakeInfo struct {
	name string
	mode fs.FileMode
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) Mode() fs.FileMode  { return i.mode }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.mode.IsDir() }
func (i fakeInfo) Sys() any           { return nil }

func (f fakeFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	files, ok := f[dir]
	if !ok {
		return nil, fs.ErrNotExist
	}
	var out []fs.DirEntry
	for name, mode := range files {
		out = append(out, fs.FileInfoToDirEntry(fakeInfo{name, mode}))
	}
	return out, nil
}

func (f fakeFS) Stat(path string) (fs.FileInfo, error) {
	dir, name := filepath.Split(path)
	mode, ok := f[filepath.Clean(dir)][name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return fakeInfo{name, mode}, nil
}

func noop(context.Context, []string) error { return nil }

func testApp(t *testing.T, fsys fakeFS, path string, opts ...Option) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	opts = append([]Option{WithPath(path), WithFS(fsys), WithStdio(strings.NewReader(""), &out, &errOut), WithEnv([]string{})}, opts...)
	return New("mtm", nil, opts...), &out, &errOut
}

func TestDiscoverFirstOnPathWinsAndSkipsNonExecutables(t *testing.T) {
	fsys := fakeFS{
		"/a": {"mtm-web": 0o755, "mtm-notes": 0o644, "mtm-dir": fs.ModeDir | 0o755, "mtm-": 0o755, "other-web": 0o755},
		"/b": {"mtm-web": 0o755, "mtm-ai": 0o755, "mtm-ai-skills": 0o700},
	}
	found, shadowed := Discover("mtm", "/a:/missing:/b:/a:", fsys)
	want := []External{{"web", "/a/mtm-web", "mtm"}, {"ai", "/b/mtm-ai", "mtm"}, {"ai-skills", "/b/mtm-ai-skills", "mtm"}}
	if !reflect.DeepEqual(found, want) {
		t.Fatalf("found = %+v", found)
	}
	if !reflect.DeepEqual(shadowed, []External{{"web", "/b/mtm-web", "mtm"}}) {
		t.Fatalf("shadowed = %+v", shadowed)
	}
	if dirs := PathDirs("/a::/b/../a:/c"); !reflect.DeepEqual(dirs, []string{"/a", ".", "/c"}) {
		t.Fatalf("dirs = %v", dirs)
	}
}

func TestResolveTable(t *testing.T) {
	fsys := fakeFS{"/bin": {"mtm-web": 0o755, "mtm-task": 0o755, "mtm-ai": 0o755, "mtm-ai-skills": 0o755}}
	app, _, _ := testApp(t, fsys, "/bin")
	app.Add(
		Command{Name: "task", Aliases: []string{"t"}, Run: noop},
		Command{Name: "tags", Run: noop},
		Command{Name: "panes", Aliases: []string{"web"}, Run: noop},
	)
	cat := app.Catalog()
	cases := []struct {
		word, entry string
		kind        Kind
		exact, amb  bool
		candidates  []string
	}{
		{word: "task", entry: "task", kind: KindInline, exact: true},
		{word: "t", entry: "task", kind: KindInline, exact: true},
		{word: "tas", entry: "task", kind: KindInline},
		{word: "ta", amb: true, candidates: []string{"tags", "task"}},
		{word: "w", entry: "web", kind: KindExternal},
		{word: "web", entry: "web", kind: KindExternal, exact: true},
		{word: "a", entry: "ai", kind: KindExternal},
		{word: "ai-", entry: "ai-skills", kind: KindExternal},
		{word: "pa", entry: "panes", kind: KindInline},
		{word: "zzz"},
		{word: ""},
	}
	for _, c := range cases {
		res := cat.Resolve(c.word)
		got := ""
		if res.Entry != nil {
			got = res.Entry.Name
			if res.Entry.Kind != c.kind {
				t.Errorf("%q kind = %s", c.word, res.Entry.Kind)
			}
		}
		if got != c.entry || res.Exact != c.exact || res.Ambiguous != c.amb {
			t.Errorf("%q = %q exact %v ambiguous %v", c.word, got, res.Exact, res.Ambiguous)
		}
		if c.amb {
			var names []string
			for _, e := range res.Candidates {
				names = append(names, e.Name)
			}
			if !reflect.DeepEqual(names, c.candidates) {
				t.Errorf("%q candidates = %v", c.word, names)
			}
		}
	}
	task := cat.Lookup("task")
	if len(task.Shadowed) != 1 || task.Shadowed[0] != (Claim{Name: "task", Kind: KindExternal, Source: "/bin/mtm-task"}) {
		t.Fatalf("task shadowed = %+v", task.Shadowed)
	}
	web := cat.Lookup("web")
	if web.Kind != KindExternal || len(web.Shadowed) != 1 || web.Shadowed[0].AliasOf != "panes" {
		t.Fatalf("web = %+v", web)
	}
	if panes := cat.Lookup("panes"); len(panes.Aliases) != 0 {
		t.Fatalf("panes kept the shadowed alias: %+v", panes.Aliases)
	}
}

func TestLaterInlineRegistrationIsShadowed(t *testing.T) {
	app, _, _ := testApp(t, fakeFS{}, "")
	app.Add(Command{Name: "x", Source: "first", Run: noop}, Command{Name: "x", Source: "second", Run: noop})
	e := app.Catalog().Lookup("x")
	if e.Source != "first" || len(e.Shadowed) != 1 || e.Shadowed[0].Source != "second" {
		t.Fatalf("x = %+v", e)
	}
}

func TestHelpGolden(t *testing.T) {
	fsys := fakeFS{"/bin": {"mtm-web": 0o755, "mtm-task": 0o755}, "/opt": {"mtm-web": 0o755}}
	app, out, _ := testApp(t, fsys, "/bin:/opt")
	app.Add(
		Command{Name: "task", Aliases: []string{"t"}, Summary: "tasks", Source: "app.go:10", Run: noop},
		Command{Name: "project", Summary: "projects", Source: "app.go:20", Run: noop},
		Command{Name: "other", Aliases: []string{"task"}, Source: "app.go:30", Run: noop},
	)
	app.Help(out)
	golden := filepath.Join("testdata", "help.golden")
	if *update {
		if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(want) {
		t.Fatalf("help:\n%s\nwant:\n%s", out, want)
	}
}

func TestRunInlineDefaultAndAmbiguous(t *testing.T) {
	app, _, errOut := testApp(t, fakeFS{}, "")
	var ran, defaulted []string
	app.Add(Command{Name: "task", Run: func(_ context.Context, args []string) error { ran = args; return nil }},
		Command{Name: "tags", Run: noop})
	app.Args = []string{"tas", "launch", "--yes"}
	if err := app.Run(context.Background()); err != nil || !reflect.DeepEqual(ran, []string{"launch", "--yes"}) {
		t.Fatalf("prefix run: %v %v", err, ran)
	}
	app.Args = []string{"ta"}
	if err := app.Run(context.Background()); ExitCode(err) != 1 || !strings.Contains(errOut.String(), `"ta" is ambiguous`) || !strings.Contains(errOut.String(), "tags") {
		t.Fatalf("ambiguous: %v\n%s", err, errOut)
	}
	errOut.Reset()
	app.Args = []string{"nope", "x"}
	if err := app.Run(context.Background()); ExitCode(err) != 1 || !strings.Contains(errOut.String(), `unknown command "nope"`) {
		t.Fatalf("unknown without default: %v\n%s", err, errOut)
	}
	app.Default = func(_ context.Context, args []string) error { defaulted = args; return nil }
	if err := app.Run(context.Background()); err != nil || !reflect.DeepEqual(defaulted, []string{"nope", "x"}) {
		t.Fatalf("default: %v %v", err, defaulted)
	}
	app.Args = nil
	if err := app.Run(context.Background()); err != nil || len(defaulted) != 0 {
		t.Fatalf("default with no args: %v %v", err, defaulted)
	}
}

func TestBuiltins(t *testing.T) {
	fsys := fakeFS{"/bin": {"mtm-web": 0o755}}
	app, out, errOut := testApp(t, fsys, "/bin", WithBuiltins())
	app.Add(Command{Name: "task", Summary: "tasks", Source: "app.go:1", Run: noop})
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}} {
		out.Reset()
		app.Args = args
		if err := app.Run(context.Background()); err != nil || !strings.Contains(out.String(), "web") || !strings.Contains(out.String(), "task") {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	out.Reset()
	app.Args = []string{"commands"}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("commands: %v\n%s", err, out)
	}
	names := map[string]string{}
	for _, r := range rows {
		names[r["name"].(string)] = r["kind"].(string)
	}
	if names["web"] != "external" || names["task"] != "inline" || names["which"] != "inline" {
		t.Fatalf("commands = %v", names)
	}
	out.Reset()
	app.Args = []string{"which", "we"}
	if err := app.Run(context.Background()); err != nil || out.String() != "web external /bin/mtm-web\n" {
		t.Fatalf("which: %v %q", err, out)
	}
	app.Args = []string{"which", "zz"}
	if err := app.Run(context.Background()); ExitCode(err) != 1 {
		t.Fatalf("which unknown: %v", err)
	}
	_ = errOut
}

func TestExternalRunsWithArgsEnvAndExitCode(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"args=$* parent=$ODEN_PARENT root=$ODEN_ROOT keep=$KEEP\"\nexit 3\n"
	if err := os.WriteFile(filepath.Join(dir, "mtm-ai-skills"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	parent := New("mtm", nil, WithPath(dir), WithEnv([]string{"KEEP=1"}), WithStdio(strings.NewReader(""), &out, &out))
	child := Child(parent, "ai", []string{"sk", "a", "b"})
	if child.Name != "mtm-ai" || child.RootName() != "mtm" {
		t.Fatalf("child = %s root %s", child.Name, child.RootName())
	}
	err := child.Run(context.Background())
	if ExitCode(err) != 3 || out.String() != "args=a b parent=mtm-ai root=mtm keep=1\n" {
		t.Fatalf("exit %d (%v), out %q", ExitCode(err), err, out.String())
	}
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Err != nil {
		t.Fatalf("err = %#v", err)
	}
}

func TestNewTakesRootFromTheEnvironmentForANestedName(t *testing.T) {
	env := []string{"ODEN_ROOT=mtm"}
	if a := New("mtm-ai", nil, WithEnv(env)); a.RootName() != "mtm" {
		t.Fatalf("nested root = %s", a.RootName())
	}
	if a := New("git", nil, WithEnv(env)); a.RootName() != "git" {
		t.Fatalf("unrelated root = %s", a.RootName())
	}
}

func TestReportSkipsSilentExits(t *testing.T) {
	var w bytes.Buffer
	if Report(&w, "x", &ExitError{Code: 4}) != 4 || w.Len() != 0 {
		t.Fatalf("silent: %q", w.String())
	}
	if Report(&w, "x", errors.New("boom")) != 1 || w.String() != "x: boom\n" {
		t.Fatalf("loud: %q", w.String())
	}
	if Report(&w, "x", nil) != 0 {
		t.Fatal("nil")
	}
}

func TestAddRecordsTheCallerAsSource(t *testing.T) {
	app, _, _ := testApp(t, fakeFS{}, "")
	app.Add(Command{Name: "x", Run: noop})
	if src := app.Catalog().Lookup("x").Source; !strings.HasPrefix(src, "oden/oden_test.go:") {
		t.Fatalf("source = %q", src)
	}
}

// The owner's nesting example: horror finds horror-biographies, which as an
// oden app finds horror-biographies-freddie-kruger. horror's own scan also
// lists biographies-freddie-kruger (as git would), but a descendant never
// makes a prefix ambiguous.
func TestNestedBinsResolveThroughDescendants(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"horror-biographies":                "echo \"biographies: $* parent=$ODEN_PARENT\"",
		"horror-biographies-freddie-kruger": "echo \"freddie: $*\"",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	horror := New("horror", []string{"bio", "fre"}, WithPath(dir), WithEnv([]string{}), WithStdio(strings.NewReader(""), &out, &out))
	if e := horror.Catalog().Lookup("biographies-freddie-kruger"); e == nil || e.Kind != KindExternal {
		t.Fatalf("the descendant is not listed: %+v", horror.Catalog().Entries)
	}
	res := horror.Catalog().Resolve("bio")
	if res.Ambiguous || res.Entry == nil || res.Entry.Path != filepath.Join(dir, "horror-biographies") {
		t.Fatalf("bio = %+v", res)
	}
	if err := horror.Run(context.Background()); err != nil || out.String() != "biographies: fre parent=horror\n" {
		t.Fatalf("horror bio fre: %v %q", err, out.String())
	}
	out.Reset()
	child := Child(horror, "biographies", []string{"fre", "x"})
	if res := child.Catalog().Resolve("fre"); res.Entry == nil || res.Entry.Path != filepath.Join(dir, "horror-biographies-freddie-kruger") {
		t.Fatalf("child fre = %+v", res)
	}
	if err := child.Run(context.Background()); err != nil || out.String() != "freddie: x\n" {
		t.Fatalf("horror-biographies fre x: %v %q", err, out.String())
	}
}

func TestSiblingPrefixIsStillAmbiguous(t *testing.T) {
	fsys := fakeFS{"/bin": {"horror-biographies": 0o755, "horror-biographies-freddie-kruger": 0o755, "horror-books": 0o755}}
	var out, errOut bytes.Buffer
	horror := New("horror", []string{"b"}, WithPath("/bin"), WithFS(fsys), WithStdio(strings.NewReader(""), &out, &errOut))
	err := horror.Run(context.Background())
	if ExitCode(err) != 1 || !strings.Contains(errOut.String(), "biographies") || !strings.Contains(errOut.String(), "books") {
		t.Fatalf("horror b: %v\n%s", err, errOut.String())
	}
	var names []string
	for _, e := range horror.Catalog().Resolve("b").Candidates {
		names = append(names, e.Name)
	}
	if !reflect.DeepEqual(names, []string{"biographies", "books"}) {
		t.Fatalf("candidates = %v", names)
	}
}

func TestInlineDescendantsFollowTheSameRule(t *testing.T) {
	app, _, _ := testApp(t, fakeFS{}, "")
	app.Add(Command{Name: "ai", Run: noop}, Command{Name: "ai-skills", Run: noop}, Command{Name: "ai-models", Run: noop})
	if res := app.Catalog().Resolve("a"); res.Entry == nil || res.Entry.Name != "ai" {
		t.Fatalf("a = %+v", res)
	}
	if res := app.Catalog().Resolve("ai-"); !res.Ambiguous || len(res.Candidates) != 2 {
		t.Fatalf("ai- = %+v", res)
	}
}

// WithPrefixes: the bin's own <bin>-<sub> beats an extra prefix's
// <extra>-<sub> wherever each sits on PATH; the loser is shadowed.
func TestPrefixesOwnBeatsExtraAndHelpShowsThePath(t *testing.T) {
	fsys := fakeFS{
		"/a": {"o-proj": 0o755, "o-web": 0o755, "x-web": 0o755},
		"/b": {"mtm-web": 0o755, "o-proj": 0o755},
	}
	app, out, _ := testApp(t, fsys, "/a:/b", WithPrefixes("o", "x"))
	cat := app.Catalog()
	proj := cat.Lookup("proj")
	if proj == nil || proj.Kind != KindExternal || proj.Path != "/a/o-proj" || proj.Prefix != "o" {
		t.Fatalf("proj = %+v", proj)
	}
	if len(proj.Shadowed) != 1 || proj.Shadowed[0].Source != "/b/o-proj" {
		t.Fatalf("proj shadowed = %+v", proj.Shadowed)
	}
	web := cat.Lookup("web")
	if web.Path != "/b/mtm-web" || web.Prefix != "mtm" {
		t.Fatalf("web = %+v", web)
	}
	var lost []string
	for _, c := range web.Shadowed {
		lost = append(lost, c.Source)
	}
	if !reflect.DeepEqual(lost, []string{"/a/o-web", "/a/x-web"}) {
		t.Fatalf("web shadowed = %v", lost)
	}
	app.Help(out)
	for _, want := range []string{"proj  (external)", "/a/o-proj", "shadows external /b/o-proj", "shadows external /a/o-web"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help lacks %q:\n%s", want, out)
		}
	}
	plain, _, _ := testApp(t, fsys, "/a:/b")
	if plain.Catalog().Lookup("proj") != nil {
		t.Fatal("extra prefixes are opt-in")
	}
}

func TestPrefixesKeepTheDescendantRule(t *testing.T) {
	fsys := fakeFS{"/bin": {"mtm-ai": 0o755, "o-ai-skills": 0o755, "o-alpha": 0o755}}
	app, _, _ := testApp(t, fsys, "/bin", WithPrefixes("o"))
	cat := app.Catalog()
	if res := cat.Resolve("ai"); !res.Exact || res.Entry.Path != "/bin/mtm-ai" {
		t.Fatalf("ai = %+v", res)
	}
	if res := cat.Resolve("ai-"); res.Entry == nil || res.Entry.Path != "/bin/o-ai-skills" {
		t.Fatalf("ai- = %+v", res)
	}
	if res := cat.Resolve("a"); !res.Ambiguous || len(res.Candidates) != 2 {
		t.Fatalf("a = %+v", res)
	}
	fsys["/bin"]["o-alpha"] = 0o644
	app, _, _ = testApp(t, fsys, "/bin", WithPrefixes("o"))
	if res := app.Catalog().Resolve("a"); res.Entry == nil || res.Entry.Name != "ai" {
		t.Fatalf("a (descendant only) = %+v", res)
	}
}

// An app's own catalog and state replace the builtins whether added before
// or after WithBuiltins runs (cli.Mount adds catalog after New); the
// builtin is the claim each shadows.
func TestAppCommandsReplaceBuiltins(t *testing.T) {
	var out bytes.Buffer
	own := func(word string) Command {
		return Command{Name: word, Source: "app.go:1", Run: func(context.Context, []string) error {
			out.WriteString("own " + word)
			return nil
		}}
	}
	for _, before := range []bool{false, true} {
		var opts []Option
		if before {
			opts = append(opts, func(a *App) { a.Add(own("catalog")) })
		}
		app, _, _ := testApp(t, fakeFS{}, "", append(opts, WithBuiltins())...)
		if !before {
			app.Add(own("catalog"))
		}
		app.Add(own("state"))
		for _, word := range []string{"catalog", "state"} {
			out.Reset()
			app.Args = []string{word}
			if err := app.Run(context.Background()); err != nil || out.String() != "own "+word {
				t.Fatalf("before=%v %s: %v %q", before, word, err, out.String())
			}
			if e := app.Catalog().Lookup(word); e.Source != "app.go:1" || len(e.Shadowed) != 1 || !strings.HasPrefix(e.Shadowed[0].Source, "oden/builtins.go:") {
				t.Fatalf("before=%v %s entry %+v", before, word, e)
			}
		}
		if e := app.Catalog().Lookup("help"); e == nil || e.Kind != KindInline {
			t.Fatalf("help builtin lost: %+v", e)
		}
	}
}
