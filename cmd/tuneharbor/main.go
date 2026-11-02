package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/theng12/tuneharbor/internal/desktop"
	"github.com/theng12/tuneharbor/internal/profiles"
	"golang.org/x/term"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Music Studio KH:", err)
		os.Exit(1)
	}
}
func run(args []string, input io.Reader, output io.Writer) error {
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	global := flag.NewFlagSet("tuneharbor", flag.ContinueOnError)
	global.SetOutput(output)
	root := global.String("data", filepath.Join(base, "TuneHarbor", "profiles"), "profile data directory")
	if err = global.Parse(args); err != nil {
		return err
	}
	args = global.Args()
	if len(args) == 0 {
		fmt.Fprintln(output, "Music Studio KH · Your music, at home\nCommands: profile create, profile list, serve\nUse --data PATH before the command to change profile storage.")
		return nil
	}
	switch args[0] {
	case "profile":
		if len(args) < 2 {
			return errors.New("use profile create or profile list")
		}
		switch args[1] {
		case "list":
			items, e := profiles.List(*root)
			if e != nil {
				return e
			}
			for _, p := range items {
				fmt.Fprintf(output, "%s  %s  port=%d  user=%s  library=%s\n", p.ID, p.Name, p.Port, p.Username, p.Library)
			}
			return nil
		case "create":
			f := flag.NewFlagSet("profile create", flag.ContinueOnError)
			f.SetOutput(output)
			name := f.String("name", "Personal", "profile name")
			music := f.String("library", "", "existing music folder")
			username := f.String("username", "owner", "username (case-insensitive)")
			port := f.Int("port", 4747, "listening port")
			lan := f.Bool("lan", false, "allow local-network access (HTTP; trusted networks only)")
			passwordFile := f.String("password-file", "", "read password from a private file instead of prompting")
			if e := f.Parse(args[2:]); e != nil {
				return e
			}
			if *music == "" {
				return errors.New("provide --library with an existing music folder")
			}
			var password string
			if *passwordFile != "" {
				data, e := os.ReadFile(*passwordFile)
				if e != nil {
					return e
				}
				password = strings.TrimRight(string(data), "\r\n")
			} else {
				fmt.Fprint(output, "Password (at least 6 characters; numbers are fine): ")
				if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
					data, e := term.ReadPassword(int(file.Fd()))
					fmt.Fprintln(output)
					if e != nil {
						return e
					}
					password = string(data)
				} else {
					line, e := bufio.NewReader(input).ReadString('\n')
					if e != nil && e != io.EOF {
						return e
					}
					password = strings.TrimRight(line, "\r\n")
				}
			}
			p, e := profiles.Create(*root, profiles.Input{Name: *name, Library: *music, Username: *username, Password: password, Port: *port, LAN: *lan})
			if e != nil {
				return e
			}
			fmt.Fprintf(output, "Created %s (%s), port %d. Run tuneharbor serve to start.\n", p.Name, p.ID, p.Port)
			return nil
		default:
			return errors.New("unknown profile command")
		}
	case "desktop":
		return runDesktop(*root, args[1:], input, output)
	case "serve":
		flags := flag.NewFlagSet("serve", flag.ContinueOnError)
		toolDir := flags.String("tools", "", "optional bundled runtime tool folder")
		if e := flags.Parse(args[1:]); e != nil {
			return e
		}
		return serveTools(*root, *toolDir, output)
	default:
		return errors.New("unknown command; use profile create, profile list, or serve")
	}
}
func serve(root string, output io.Writer) error { return serveTools(root, "", output) }
func serveTools(root, toolDir string, output io.Writer) error {
	items, e := profiles.List(root)
	if e != nil {
		return e
	}
	if len(items) == 0 {
		return errors.New("create a profile first with profile create --library PATH")
	}
	tools := desktop.Tools{}
	if toolDir != "" {
		tools = desktop.ToolPaths(toolDir)
	}
	manager := desktop.New(root, tools, nil)
	defer manager.Close()
	if e = manager.Doctor(); e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	started := 0
	for _, p := range items {
		opened, e := manager.Start(p.ID)
		if e != nil {
			fmt.Fprintf(output, "%s: could not start: %s\n", p.Name, e)
			continue
		}
		started++
		fmt.Fprintf(output, "%s → %s (username: %s)\n", p.Name, opened.URL, p.Username)
	}
	if started == 0 {
		return errors.New("no profile could start; check folders and ports")
	}
	<-ctx.Done()
	manager.Close()
	fmt.Fprintln(output, "All profile servers stopped. Music files preserved.")
	return nil
}
