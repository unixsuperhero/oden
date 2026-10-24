// Build this source under the names listed in README's delegation examples.
// Each executable is an independent oden app, selected here by its filename
// so the examples can share one source file.
package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/unixsuperhero/oden"
)

func main() {
	name := filepath.Base(os.Args[0])
	app := oden.New(name, os.Args[1:], oden.WithBuiltins())
	switch name {
	case "toolbox-files":
		app.Cmd("info", "show invocation", func(c *oden.Ctx) error {
			return c.Printf("bin=%s parent=%s root=%s arg=%s\n", c.App.Name, os.Getenv(oden.EnvParent), c.App.RootName(), c.Arg(0, "none"))
		})
	case "toolbox-files-text":
		app.Cmd("show", "print files or stdin", func(c *oden.Ctx) error {
			text, err := c.Text()
			if err != nil {
				return err
			}
			return c.Printf("%s", text)
		})
	case "desk":
		oden.WithPrefixes("toolbox")(app)
	}
	os.Exit(oden.Report(os.Stderr, name, app.Run(context.Background())))
}
