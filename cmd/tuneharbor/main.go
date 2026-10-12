package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/theng12/tuneharbor/internal/library"
	"github.com/theng12/tuneharbor/internal/profiles"
	"github.com/theng12/tuneharbor/internal/server"
	"golang.org/x/term"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "TuneHarbor:", err)
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
		fmt.Fprintln(output, "TuneHarbor · Your music, at home\nCommands: profile create, profile list, serve\nUse --data PATH before the command to change profile storage.")
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
		return serve(*root, output)
	default:
		return errors.New("unknown command; use profile create, profile list, or serve")
	}
}
func serve(root string, output io.Writer) error {
	items, e := profiles.List(root)
	if e != nil {
		return e
	}
	if len(items) == 0 {
		return errors.New("create a profile first with profile create --library PATH")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	type running struct {
		http    *http.Server
		library *library.Library
	}
	instances := []running{}
	for _, p := range items {
		l, e := library.Open(p.DataDir, p.Library)
		if e != nil {
			fmt.Fprintf(output, "%s: cannot open library; check folder access\n", p.Name)
			continue
		}
		if e = l.Scan(); e != nil {
			fmt.Fprintf(output, "%s: scan failed; previous index preserved\n", p.Name)
		}
		host := "127.0.0.1"
		if p.LAN {
			host = "0.0.0.0"
		}
		listener, e := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p.Port))
		if e != nil {
			l.Close()
			fmt.Fprintf(output, "%s: port %d unavailable; other profiles can still start\n", p.Name, p.Port)
			continue
		}
		h := &http.Server{Handler: server.New(p, l).Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
		instances = append(instances, running{h, l})
		fmt.Fprintf(output, "%s → http://127.0.0.1:%d (username: %s)\n", p.Name, p.Port, p.Username)
		if p.LAN {
			addresses, _ := net.InterfaceAddrs()
			for _, a := range addresses {
				if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
					fmt.Fprintf(output, "  LAN → http://%s:%d (HTTP; trusted networks only)\n", n.IP.String(), p.Port)
				}
			}
		}
		go func() {
			if err := h.Serve(listener); err != nil && err != http.ErrServerClosed {
				fmt.Fprintln(output, "Profile server stopped unexpectedly:", err)
			}
		}()
		go func() {
			ticker := time.NewTicker(20 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := l.Scan(); err != nil {
						fmt.Fprintln(output, "Library refresh failed; keeping last index")
					}
				}
			}
		}()
	}
	if len(instances) == 0 {
		return errors.New("no profile could start; check folders and ports")
	}
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, instance := range instances {
		if err := instance.http.Shutdown(shutdown); err != nil {
			instance.http.Close()
		}
		instance.library.Close()
	}
	fmt.Fprintln(output, "All profile servers stopped. Music files preserved.")
	return nil
}
