// Command openintern is the single binary for everything OpenIntern runs:
// ingest, dumps, the website and API, registry checks and maintenance tools.
//
//	openintern <command> [flags]
//
// Each workstream registers its commands from its own cmd_<area>.go file with
// register(), so files never collide. Run `openintern help` for the list.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
)

type command struct {
	name    string
	summary string
	run     func(args []string) error
}

var commands = map[string]command{}

// register adds a subcommand. Call it from an init() in cmd_<area>.go.
func register(name, summary string, run func(args []string) error) {
	if _, dup := commands[name]; dup {
		panic("openintern: duplicate command " + name)
	}
	commands[name] = command{name: name, summary: summary, run: run}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: openintern <command> [flags]\n\ncommands:")
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(os.Stderr, "  %-24s %s\n", n, commands[n].summary)
	}
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" {
		usage()
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
	}
	cmd, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "openintern: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err := cmd.run(os.Args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "openintern %s: %v\n", cmd.name, err)
		os.Exit(1)
	}
}
