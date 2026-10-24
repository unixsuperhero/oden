// Command o is the family root bin (oden, ochazuke): every o-<tool> on
// PATH is o's command <tool> by oden's normal rule (o proj new x runs
// o-proj new x with ODEN_PARENT=o), plus oden's builtins. Nothing else is
// built in: the TUI host (nabe) is installed as o-nabe and o-tui by
// convention.
package main

import (
	"context"
	"os"

	"github.com/unixsuperhero/oden"
)

func newApp(args []string, opts ...oden.Option) *oden.App {
	return oden.New("o", args, append([]oden.Option{oden.WithBuiltins()}, opts...)...)
}

func main() {
	os.Exit(oden.Report(os.Stderr, "o", newApp(os.Args[1:]).Run(context.Background())))
}
