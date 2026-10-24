// Command o is the family root bin (oden, ochazuke): every o-<tool> on
// PATH is o's command <tool> by oden's normal rule (o proj new x runs
// o-proj new x with ODEN_PARENT=o), plus oden's builtins and tui, which
// execs nabe with the remaining words. o holds no TUI code itself.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/unixsuperhero/oden"
)

// runner is how o starts nabe: lookPath finds it, execve replaces o with
// it. Tests inject both.
type runner struct {
	lookPath func(file string) (string, error)
	execve   func(path string, argv, env []string) error
	environ  func() []string
}

var osRunner = runner{lookPath: exec.LookPath, execve: syscall.Exec, environ: os.Environ}

func newApp(args []string, r runner, opts ...oden.Option) *oden.App {
	app := oden.New("o", args, append([]oden.Option{oden.WithBuiltins()}, opts...)...)
	app.Add(oden.Command{Name: "tui", Summary: "open nabe, the extension host TUI", Run: func(_ context.Context, words []string) error {
		path, err := r.lookPath("nabe")
		if err != nil {
			return fmt.Errorf("tui: nabe is not on PATH (go install github.com/unixsuperhero/nabe@latest): %w", err)
		}
		return r.execve(path, append([]string{"nabe"}, words...), r.environ())
	}})
	return app
}

func main() {
	os.Exit(oden.Report(os.Stderr, "o", newApp(os.Args[1:], osRunner).Run(context.Background())))
}
