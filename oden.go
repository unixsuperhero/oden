// Package oden is a small framework for extensible subcommand-driven CLI
// tools: a bin's inline commands merged with the executables named
// <bin>-<sub> on PATH (git- and hiiro-style third-party commands), resolved
// by exact name, alias or unique prefix. It knows no application.
package oden

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Environment variables an external command is started with.
const (
	EnvParent = "ODEN_PARENT" // the bin that ran it
	EnvRoot   = "ODEN_ROOT"   // the outermost bin of the chain
)

// Command is an inline command. Run receives the words after the command's
// own. Source is where it was registered (file:line), filled by Add.
type Command struct {
	Name    string
	Aliases []string
	Summary string
	Run     func(ctx context.Context, args []string) error
	Source  string
}

// App is one bin: its name, its arguments (without the bin name), its
// inline commands and where it looks for external ones.
type App struct {
	Name string
	Root string // the outermost bin; "" is Name
	Args []string

	// Default runs when no command word is given or the word names no
	// command; it receives every argument. Without it, help is printed to
	// Stderr and Run exits 1.
	Default func(ctx context.Context, args []string) error

	Path func() string // the PATH list; nil reads $PATH
	FS   FS            // nil is OS
	Env  []string      // an external's environment; nil is os.Environ()

	// Prefixes (WithPrefixes) are extra bin names whose <name>-<sub>
	// executables are discovered after the bin's own.
	Prefixes []string

	Stdin          io.Reader
	Stdout, Stderr io.Writer

	commands []*Command
	catalog  *Catalog
}

// Option configures an App in New.
type Option func(*App)

// New builds an App named name over args. Root is taken from ODEN_ROOT
// when name is nested under it (an external run by an oden parent).
func New(name string, args []string, opts ...Option) *App {
	a := &App{Name: name, Args: args, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	for _, opt := range opts {
		opt(a)
	}
	if a.Root == "" {
		if root := a.getenv(EnvRoot); root != "" && strings.HasPrefix(name, root+"-") {
			a.Root = root
		}
	}
	return a
}

// WithPath fixes the PATH list discovery reads.
func WithPath(path string) Option { return func(a *App) { a.Path = func() string { return path } } }

// WithFS sets the file system discovery reads.
func WithFS(fsys FS) Option { return func(a *App) { a.FS = fsys } }

// WithEnv sets the environment externals start with.
func WithEnv(env []string) Option { return func(a *App) { a.Env = env } }

// WithStdio sets the streams commands read and write.
func WithStdio(in io.Reader, out, errOut io.Writer) Option {
	return func(a *App) { a.Stdin, a.Stdout, a.Stderr = in, out, errOut }
}

// WithPrefixes adds bin names whose <name>-<sub> executables on PATH are
// also this bin's commands, discovered after its own <bin>-<sub> and in
// the given order: with WithPrefixes("o"), mtm proj runs o-proj unless an
// mtm-proj is on PATH, which then shadows it. A Child does not inherit
// them.
func WithPrefixes(names ...string) Option {
	return func(a *App) { a.Prefixes = append(a.Prefixes, names...) }
}

// WithDefault sets the handler for no command word or an unknown one.
func WithDefault(run func(ctx context.Context, args []string) error) Option {
	return func(a *App) { a.Default = run }
}

// Child is the App for parent's command sub: named <parent>-<sub>, so its
// own externals are <parent>-<sub>-<word>, sharing the parent's root,
// PATH, file system, environment and streams.
func Child(parent *App, sub string, args []string) *App {
	return &App{
		Name: parent.Name + "-" + sub, Root: parent.RootName(), Args: args,
		Path: parent.Path, FS: parent.FS, Env: parent.Env,
		Stdin: parent.Stdin, Stdout: parent.Stdout, Stderr: parent.Stderr,
	}
}

// RootName is the outermost bin of the chain.
func (a *App) RootName() string {
	if a.Root != "" {
		return a.Root
	}
	return a.Name
}

// Add registers inline commands, recording the caller as their Source
// when none is set.
func (a *App) Add(cmds ...Command) *App { return a.add(2, cmds...) }

func (a *App) add(skip int, cmds ...Command) *App {
	source := ""
	if _, file, line, ok := runtime.Caller(skip); ok {
		source = fmt.Sprintf("%s:%d", shortFile(file), line)
	}
	for _, c := range cmds {
		if c.Source == "" {
			c.Source = source
		}
		a.commands = append(a.commands, &c)
	}
	a.catalog = nil
	return a
}

// shortFile is a source file's last directory and name.
func shortFile(file string) string {
	return filepath.Join(filepath.Base(filepath.Dir(file)), filepath.Base(file))
}

// Catalog is the merged inline and external commands, discovered on first
// use and cached until the next Add.
func (a *App) Catalog() *Catalog {
	if a.catalog == nil {
		fsys := a.FS
		if fsys == nil {
			fsys = OS
		}
		found, shadowed := DiscoverAll(append([]string{a.Name}, a.Prefixes...), a.pathList(), fsys)
		a.catalog = NewCatalog(a.Name, a.commands, found, shadowed)
	}
	return a.catalog
}

func (a *App) pathList() string {
	if a.Path != nil {
		return a.Path()
	}
	return os.Getenv("PATH")
}

func (a *App) env() []string {
	if a.Env != nil {
		return a.Env
	}
	return os.Environ()
}

func (a *App) getenv(key string) string {
	for _, kv := range a.env() {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v
		}
	}
	return ""
}

// ExitError carries a process exit code; Err (may be nil) is the cause.
// A nil Err means the message, if any, was already printed.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitError) Unwrap() error { return e.Err }

// ExitCode is the process exit code for err: 0 for nil, an ExitError's
// code, else 1.
func ExitCode(err error) int {
	var ee *ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.Code
	}
	return 1
}

// Run resolves the first argument and runs what it names with the rest:
// an inline command in process, an external with inherited streams. No
// word or an unknown one runs Default (or prints help and exits 1); an
// ambiguous prefix prints the candidates and exits 1.
func (a *App) Run(ctx context.Context) error {
	if len(a.Args) == 0 {
		return a.fallback(ctx, "")
	}
	res := a.Catalog().Resolve(a.Args[0])
	switch {
	case res.Ambiguous:
		fmt.Fprintf(a.Stderr, "%s: %q is ambiguous; did you mean one of these?\n", a.Name, res.Query)
		a.List(a.Stderr, res.Candidates)
		return &ExitError{Code: 1}
	case !res.Resolved():
		return a.fallback(ctx, res.Query)
	}
	return a.RunEntry(ctx, res.Entry, a.Args[1:])
}

func (a *App) fallback(ctx context.Context, word string) error {
	if a.Default != nil {
		return a.Default(ctx, a.Args)
	}
	if word == "" {
		fmt.Fprintf(a.Stderr, "usage: %s COMMAND [ARGS...]\n\ncommands:\n", a.Name)
	} else {
		fmt.Fprintf(a.Stderr, "%s: unknown command %q\n\ncommands:\n", a.Name, word)
	}
	a.List(a.Stderr, a.Catalog().Entries)
	return &ExitError{Code: 1}
}

// RunEntry runs one catalog entry with args.
func (a *App) RunEntry(ctx context.Context, e *Entry, args []string) error {
	if e.Kind == KindInline {
		return e.Command.Run(ctx, args)
	}
	cmd := exec.CommandContext(ctx, e.Path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.Stdin, a.Stdout, a.Stderr
	cmd.Env = append(append([]string{}, a.env()...), EnvParent+"="+a.Name, EnvRoot+"="+a.RootName())
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return &ExitError{Code: ee.ExitCode()}
	}
	return err
}

// Report prints err as "prefix: err" to w, unless it is an ExitError with
// no cause (its message was already printed), and returns the exit code.
func Report(w io.Writer, prefix string, err error) int {
	var ee *ExitError
	if err != nil && !(errors.As(err, &ee) && ee.Err == nil) {
		fmt.Fprintf(w, "%s: %v\n", prefix, err)
	}
	return ExitCode(err)
}
