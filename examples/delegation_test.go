package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDelegationExamples(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	binary := filepath.Join(dir, "toolbox")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "delegation.go")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"toolbox-files", "toolbox-files-text", "desk"} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0700); err != nil {
			t.Fatal(err)
		}
	}
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("file input\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		bin         string
		args        []string
		stdin, want string
	}{
		{"toolbox", []string{"files", "info", "Ada"}, "", "bin=toolbox-files parent=toolbox root=toolbox arg=Ada\n"},
		{"toolbox", []string{"fi", "info", "Ada"}, "", "bin=toolbox-files parent=toolbox root=toolbox arg=Ada\n"},
		{"toolbox", []string{"files", "text", "show"}, "hello\n", "hello\n"},
		{"toolbox", []string{"files", "text", "show", input}, "", "file input\n"},
		{"desk", []string{"files", "info", "Ada"}, "", "bin=toolbox-files parent=desk root=toolbox-files arg=Ada\n"},
		{"desk", []string{"files", "text", "show"}, "shared\n", "shared\n"},
	} {
		t.Run(tc.bin+" "+strings.Join(tc.args, " "), func(t *testing.T) {
			cmd := exec.CommandContext(ctx, filepath.Join(dir, tc.bin), tc.args...)
			cmd.Env = []string{"PATH=" + dir}
			cmd.Stdin = strings.NewReader(tc.stdin)
			out, err := cmd.CombinedOutput()
			if err != nil || string(out) != tc.want {
				t.Fatalf("run = %v, %q; want %q", err, out, tc.want)
			}
		})
	}
}
