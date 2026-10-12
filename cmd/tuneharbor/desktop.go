package main

import (
	"flag"
	"github.com/theng12/tuneharbor/internal/desktop"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func runDesktop(root string, args []string, input io.Reader, output io.Writer) error {
	flags := flag.NewFlagSet("desktop", flag.ContinueOnError)
	tools := flags.String("tools", "", "bundled runtime tool directory")
	if e := flags.Parse(args); e != nil {
		return e
	}
	m := desktop.New(root, desktop.ToolPaths(*tools), nil)
	// A terminated server still stops its profiles, downloads and conversions.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	go func() {
		if _, ok := <-signals; ok {
			m.Close()
			os.Exit(0)
		}
	}()
	return desktop.Run(input, output, m)
}
