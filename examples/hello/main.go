// Command hello is the oden example: two inline commands (greet, echo) and
// any hello-<sub> executable on PATH. Try it:
//
//	go build -o /tmp/hello ./examples/hello
//	PATH="$PWD/examples/hello/bin:$PATH" /tmp/hello help
//	PATH="$PWD/examples/hello/bin:$PATH" /tmp/hello we   # runs bin/hello-web
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/unixsuperhero/oden"
)

func main() {
	app := oden.New("hello", os.Args[1:], oden.WithBuiltins())
	app.Add(
		oden.Command{Name: "greet", Aliases: []string{"hi"}, Summary: "say hello", Run: func(_ context.Context, args []string) error {
			name := "world"
			if len(args) > 0 {
				name = args[0]
			}
			fmt.Fprintf(app.Stdout, "hello, %s\n", name)
			return nil
		}},
		oden.Command{Name: "echo", Summary: "print the arguments", Run: func(_ context.Context, args []string) error {
			fmt.Fprintln(app.Stdout, strings.Join(args, " "))
			return nil
		}},
	)
	os.Exit(oden.Report(os.Stderr, "hello", app.Run(context.Background())))
}
