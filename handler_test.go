package oden

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCmdContextAndSub(t *testing.T) {
	app, out, errOut := testApp(t, fakeFS{}, "", WithPrefixes("shared"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.Catalog()
	app.Cmd("group", "nested commands", func(c *Ctx) error {
		if c.App != app || c.Name != "group" || c.Err() != context.Canceled || c.Stdout != out || c.Stderr != errOut || c.Stdin != app.Stdin {
			t.Fatalf("context = %+v", c)
		}
		sub := c.Sub(WithBuiltins())
		if sub.Name != "mtm-group" || sub.RootName() != "mtm" || len(sub.Prefixes) != 0 || sub.FS != nil && !reflect.DeepEqual(sub.FS, app.FS) {
			t.Fatalf("sub = %+v", sub)
		}
		return sub.Cmd("greet", "greet", func(c *Ctx) error {
			if c.Arg(-1, "fallback") != "fallback" || c.Arg(1, "fallback") != "fallback" {
				t.Fatal("argument defaults")
			}
			return c.Printf("hello, %s\n", c.Arg(0, "world"))
		}).Run(c)
	}, "g")
	if src := app.Catalog().Lookup("group").Source; !strings.Contains(src, "handler_test.go:") {
		t.Fatalf("source = %q", src)
	}
	for _, word := range []string{"group", "g", "gro"} {
		out.Reset()
		app.Args = []string{word, "greet", "Ada"}
		if err := app.Run(ctx); err != nil || out.String() != "hello, Ada\n" {
			t.Fatalf("run = %v, %q", err, out)
		}
	}
}

func TestHandlePrecedenceAndUsage(t *testing.T) {
	app, _, errOut := testApp(t, fakeFS{}, "")
	app.Add(Command{Name: "read", Run: func(context.Context, []string) error { t.Fatal("legacy Run called"); return nil }, Handle: func(c *Ctx) error { return c.Usage("FILE...") }})
	err := app.RunEntry(context.Background(), app.Catalog().Lookup("read"), nil)
	if ExitCode(err) != 2 || errOut.String() != "usage: mtm read FILE...\n" {
		t.Fatalf("usage = %v, %q", err, errOut)
	}
	var report bytes.Buffer
	Report(&report, "mtm", err)
	if report.Len() != 0 {
		t.Fatal("usage printed twice")
	}
}

func TestInputForms(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first"), filepath.Join(dir, "second")
	for path, text := range map[string]string{first: "first\n", second: "second\n"} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name  string
		args  []string
		input io.Reader
		want  string
	}{
		{"stdin", nil, strings.NewReader("stdin\n"), "stdin\n"},
		{"files", []string{first, second}, strings.NewReader("unused"), "first\nsecond\n"},
		{"mixed", []string{first, "-", second, "-"}, strings.NewReader("stdin\n"), "first\nstdin\nsecond\n"},
	}
	pipe, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()
	if _, err := writer.WriteString("pipe\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	cases = append(cases, struct {
		name  string
		args  []string
		input io.Reader
		want  string
	}{"pipe", nil, pipe, "pipe\n"})
	redirected, err := os.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer redirected.Close()
	cases = append(cases, struct {
		name  string
		args  []string
		input io.Reader
		want  string
	}{"redirect", nil, redirected, "first\n"})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			app := New("test", append([]string{"cat"}, tc.args...), WithPath(dir), WithEnv([]string{}), WithStdio(tc.input, &out, &out))
			app.Cmd("cat", "read input", func(c *Ctx) error {
				text, err := c.Text()
				if err != nil {
					return err
				}
				return c.Printf("%s", text)
			})
			if err := app.Run(context.Background()); err != nil || out.String() != tc.want {
				t.Fatalf("input = %v, %q", err, out.String())
			}
		})
	}
	if _, err := redirected.Stat(); err != nil {
		t.Fatalf("stdin closed: %v", err)
	}
	c := &Ctx{Args: []string{first, filepath.Join(dir, "missing"), second}}
	text, err := c.Text()
	if text != "first\n" || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file = %q, %v", text, err)
	}
	r := c.Open(first)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed read = %v", err)
	}
}

type failedIO struct{}

func (failedIO) Read([]byte) (int, error)  { return 0, io.ErrUnexpectedEOF }
func (failedIO) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestLinesAndStreamErrors(t *testing.T) {
	c := &Ctx{Stdin: strings.NewReader("one\ntwo\nlast")}
	var lines []string
	for line, err := range c.Lines() {
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, line)
	}
	if !reflect.DeepEqual(lines, []string{"one", "two", "last"}) {
		t.Fatal(lines)
	}
	c.Stdin = strings.NewReader("first\nsecond\n")
	count := 0
	for _, err := range c.Lines() {
		if err != nil {
			t.Fatal(err)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatal(count)
	}
	c.Stdin = failedIO{}
	for _, err := range c.Lines() {
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
		count++
	}
	if count != 2 {
		t.Fatal("missing read error")
	}
	c.Stdin = strings.NewReader(strings.Repeat("x", 70*1024))
	for _, err := range c.Lines() {
		if err == nil {
			t.Fatal("missing scanner error")
		}
	}
	c.Stdout = failedIO{}
	if !errors.Is(c.Println("x"), io.ErrClosedPipe) || !errors.Is(c.Printf("x"), io.ErrClosedPipe) {
		t.Fatal("lost write error")
	}
}
