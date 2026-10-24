package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/unixsuperhero/oden"
)

func TestTuiExecsNabeWithTheRemainingWords(t *testing.T) {
	var gotPath string
	var gotArgv []string
	r := runner{
		lookPath: func(file string) (string, error) {
			if file != "nabe" {
				t.Fatalf("looked up %q", file)
			}
			return "/fake/bin/nabe", nil
		},
		execve:  func(path string, argv, _ []string) error { gotPath, gotArgv = path, argv; return nil },
		environ: func() []string { return nil },
	}
	app := newApp([]string{"tu", "--smoke", "-data", "/x"}, r, oden.WithPath(""), oden.WithStdio(strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}))
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/fake/bin/nabe" || !reflect.DeepEqual(gotArgv, []string{"nabe", "--smoke", "-data", "/x"}) {
		t.Fatalf("exec %q %v", gotPath, gotArgv)
	}
}

func TestTuiWithoutNabeIsAClearError(t *testing.T) {
	r := runner{
		lookPath: func(string) (string, error) { return "", errors.New("not found") },
		execve:   func(string, []string, []string) error { t.Fatal("exec without nabe"); return nil },
		environ:  func() []string { return nil },
	}
	err := newApp([]string{"tui"}, r, oden.WithPath("")).Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "nabe is not on PATH") {
		t.Fatalf("err = %v", err)
	}
}

func TestOProjRunsOProjWithParentO(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"o-proj $* parent=$ODEN_PARENT\"\n"
	if err := os.WriteFile(filepath.Join(dir, "o-proj"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	app := newApp([]string{"proj", "new", "x"}, osRunner, oden.WithPath(dir), oden.WithEnv([]string{}), oden.WithStdio(strings.NewReader(""), &out, &out))
	if err := app.Run(context.Background()); err != nil || out.String() != "o-proj new x parent=o\n" {
		t.Fatalf("o proj new x: %v %q", err, out.String())
	}
}
