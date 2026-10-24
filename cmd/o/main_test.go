package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unixsuperhero/oden"
)

// o has no inline commands beyond the builtins: o tui and o proj are
// o-tui and o-proj on PATH, run with the remaining words and ODEN_PARENT=o.
func TestOToolsAreOPrefixedExternals(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"o-tui", "o-proj"} {
		script := "#!/bin/sh\necho \"" + name + " $* parent=$ODEN_PARENT\"\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"tui", "--smoke"}, "o-tui --smoke parent=o\n"},
		{[]string{"proj", "new", "x"}, "o-proj new x parent=o\n"},
		{[]string{"pr", "check"}, "o-proj check parent=o\n"},
	} {
		var out bytes.Buffer
		app := newApp(c.args, oden.WithPath(dir), oden.WithEnv([]string{}), oden.WithStdio(strings.NewReader(""), &out, &out))
		if err := app.Run(context.Background()); err != nil || out.String() != c.want {
			t.Fatalf("o %v: %v %q", c.args, err, out.String())
		}
	}
}
