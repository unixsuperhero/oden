// Command hello demonstrates context handlers (greet, echo, cat, lines) and
// any hello-<sub> executable on PATH. Try it:
//
//	go build -o /tmp/hello ./examples/hello
//	PATH="$PWD/examples/hello/bin:$PATH" /tmp/hello help
//	PATH="$PWD/examples/hello/bin:$PATH" /tmp/hello we   # runs bin/hello-web
package main

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/unixsuperhero/oden"
)

func main() {
	app := oden.New("hello", os.Args[1:], oden.WithBuiltins())
	app.Cmd("greet", "say hello", func(c *oden.Ctx) error {
		return c.Printf("hello, %s\n", c.Arg(0, "world"))
	}, "hi")
	app.Cmd("echo", "print the arguments", func(c *oden.Ctx) error {
		return c.Println(strings.Join(c.Args, " "))
	})
	app.Cmd("cat", "concatenate files or stdin", func(c *oden.Ctx) error {
		in := c.Input()
		defer in.Close()
		_, err := io.Copy(c.Stdout, in)
		return err
	})
	app.Cmd("lines", "number input lines", func(c *oden.Ctx) error {
		n := 0
		for line, err := range c.Lines() {
			if err != nil {
				return err
			}
			n++
			if err := c.Printf("%d: %s\n", n, line); err != nil {
				return err
			}
		}
		return nil
	})
	os.Exit(oden.Report(os.Stderr, "hello", app.Run(context.Background())))
}
