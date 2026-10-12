package main

import (
	"flag"
	"github.com/theng12/tuneharbor/internal/desktop"
	"io"
)

func runDesktop(root string, args []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("desktop", flag.ContinueOnError)
	tools := flags.String("tools", "", "bundled runtime tool directory")
	if e := flags.Parse(args); e != nil {
		return e
	}
	return desktop.Run(input, output, desktop.New(root, desktop.ToolPaths(*tools), nil))
}
