// COPYRIGHT(C) 2018-2020  David N. Cohen.
// This file is part of src.d10.dev/command
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package command provides consistency in parsing command line flags,
// reading config files, and writing logs.  This package is intended
// for launching commands with operations (sub-commands).
//
// Commands take the form:
//     command [flag ...] <operation> [operation flag ...] [arg ...]
//
// Common Operations
//
// Provides a "help" psuedo-operation, which may be invoked as:
//     command help              # shows general usage
//     command help <operation>  # shows operation-specific usage
//
// Common Flags
//
// Top level flags include "-v" for verbosity, and "-config" to
// specify the directory where configuration files are found.
//
package command

//go:generate sh -c "go doc | dumbdown > README.md"

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime/pprof"
	"sort"
	"strings"
)

type command struct {
	Name        string
	Syntax      string
	Description string

	operation map[string]operation
}

// Type Operation describes a sub-command.  See also RegisterOperation().
type operation struct {
	// handler() is to operation as main() is to command.
	handler func() error

	command
}

var (
	Command command

	// support multiple usage errors
	msgs []string

	configFlag  *string
	verboseFlag BoolCount

	// TODO(dnc): allow conditional compile with runtime/pprof
	cpuProfileFlag *string // https://blog.golang.org/profiling-go-programs
	memProfileFlag *string

	// exit status will be non-zero if error messages logged
	status int
)

type option string

const (
	OptionConfig  option = "config"
	OptionProfile option = "profile"
	OptionVerbose option = "verbose"
)

// Inject details about the current command.
func RegisterCommand(name, syntax, description string, option ...option) {
	if name == "" {
		log.Panic("cannot register command without name")
	}
	// prevent multiple calls
	if Command.Name != "" {
		log.Panicf("cannot register command (%q), command (%q) already registered", name, Command.Name)
	}

	c := command{
		Name:        name,
		Syntax:      syntax,
		Description: description,
		operation:   make(map[string]operation),
	}

	// to support RegisterCommand and RegisterOperation being called in
	// init() functions, we might have operations before we have a
	// command!
	for _, o := range Command.operation {
		c.RegisterOperation(o.handler, o.Name, o.Syntax, o.Description)
	}

	Command = c

	flag.Usage = Command.usage

	for _, o := range option {
		switch o {

		case OptionConfig:
			configFlag = flag.CommandLine.String("config", ConfigDir(), "directory where configuration files are found")

		case OptionProfile:
			// https://blog.golang.org/profiling-go-programs
			cpuProfileFlag = flag.CommandLine.String("cpuprofile", "", "write cpu profile to file")
			memProfileFlag = flag.CommandLine.String("memprofile", "", "write mem profile to file")

		case OptionVerbose:
			// if glog imported by a dependency, "v" may already be defined
			v := "v"
			for flag.CommandLine.Lookup(v) != nil {
				v = v + "v" // use "vv" if "v" is not available
			}
			flag.CommandLine.Var(&verboseFlag, v, "verbose output")

		}
	}

}

func RegisterOperation(handler func() error, name, syntax, description string) {
	Command.RegisterOperation(handler, name, syntax, description)
}

// Add an operation (sub-command) to a command.  Typically called from
// an init() function in the file defining the operation.
func (c *command) RegisterOperation(handler func() error, name, syntax, description string) {
	if name != strings.TrimSpace(name) {
		log.Panicf("bad operation name: %q", name)
	}

	if c.operation == nil {
		c.operation = make(map[string]operation)
	}

	_, ok := c.operation[name]
	if ok {
		log.Panicf("cannot re-register operation (%q)", name)
	}

	if c.Name != "" {
		// prepend command to operation syntax
		syntax = fmt.Sprintf("%s %s", c.Name, syntax)
	}
	c.operation[name] = operation{
		command: command{
			Name:        name,
			Syntax:      syntax,
			Description: description,
		},
		handler: handler,
	}
}

func Operate(name string) { Command.Operate(name) }

func (c *command) Operate(name string) {
	op, ok := c.operation[name]
	if !ok {
		CheckUsage(fmt.Errorf("unknown operation (%q)", name))
	}

	// manipulate globals in such a way that the operation handler
	// can refer to flag and os package as if the operation were the
	// top-level command

	flagset := flag.NewFlagSet(op.Name, flag.ExitOnError)
	flag.VisitAll(func(f *flag.Flag) {
		// TODO(dnc): consider a "passthrough" wrapper type that detects flag overrides
		flagset.Var(f.Value, f.Name, f.Usage)
	})

	os.Args, flag.CommandLine = flag.Args(), flagset
	flag.Usage = op.usage
	log.SetPrefix(fmt.Sprintf("%s %s", c.Name, op.Name))

	// use CheckUsage here, because an error returned from handler implies usage mistake
	CheckUsage(op.handler())
}

// Prepend error messages to Usage() output.
func UsageError(err interface{}) {
	switch e := err.(type) {
	case string:
		msgs = append(msgs, e)
	case error:
		msgs = append(msgs, e.Error())
	default:
		log.Panicf("Unexpected error type: %T (%#v)", e, e)
	}
	if err != flag.ErrHelp {
		status = 2 // status 2 when called incorrectly
	}
}

func Exit() {
	if cpuProfileFlag != nil && *cpuProfileFlag != "" {
		pprof.StopCPUProfile()
		V(1).Logf("wrote cpu profile to %q", *cpuProfileFlag)
	}
	if memProfileFlag != nil && *memProfileFlag != "" {
		f, err := os.Create(*memProfileFlag)
		if err != nil {
			Errorf("could not create memory profile: %s", err)
		}
		//runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			Errorf("could not write memory profile: %s", err)
		}
		V(1).Logf("wrote memory profile to %q", *memProfileFlag)
		f.Close()
	}
	os.Exit(status)
}

// usage can serve as the flag.Usage function.  It shows errors (if
// any) during flag parsing.  It shows available operations, as well
// as available flags.
func (c *command) usage() {
	output := flag.CommandLine.Output()

	if len(msgs) > 0 {
		fmt.Fprintln(output, "Error:")
		for _, msg := range msgs {
			fmt.Fprintf(output, "\t%s", msg)
		}
		fmt.Fprintln(output, "") // blank
	} else {
		if c.Description != "" {
			fmt.Fprintf(output, "\n%s\n", c.Description)
		}
	}

	if c.Syntax != "" {
		fmt.Fprintf(output, `
Usage:

  %s
`, c.Syntax)
	}

	if len(c.operation) > 0 {
		// sort operations to avoid random ordering of map
		operation := make([]operation, 0, len(c.operation))
		for _, op := range c.operation {
			operation = append(operation, op)
		}
		sort.Slice(operation, func(i, j int) bool {
			return operation[i].Name < operation[j].Name
		})

		fmt.Fprintln(output, "\nOperations:")
		for _, op := range operation {

			fmt.Fprintf(output, "\n  %s", op.Syntax)
			if op.Description != "" {
				fmt.Fprintf(output, "\n\t%s", op.Description)
			}
			fmt.Fprintf(output, "\n")
		}
	}

	if flag.CommandLine.NFlag() > 0 {
		fmt.Fprintf(output, `
Flags:

`)
		flag.CommandLine.PrintDefaults()
	}

	fmt.Println("") // blank

}

// parse deeply, repeating until all flags before and after operations
// have been parsed.  The goal is to parse top-level required flags,
// even if they appear after the operation name.  To do this before
// operation handler has been invoked, we need to ignore errors on
// deep parses.
func parse() error {
	arg := os.Args
	flagset := flag.CommandLine
	strict := true     // be strict about top-level flags, tolerant about operation flags
	help := false      // track whether "help" pseudo-command appears
	for len(arg) > 1 { // arg[0] is command name

		if arg[0] == "help" {
			help = true
			arg = arg[1:]
			continue
		}

		err := flagset.Parse(arg[1:])
		if err != nil && strict {
			return err
		}
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		if help {
			return flag.ErrHelp
		}

		arg = flagset.Args()
		flagset = flag.NewFlagSet("", flag.ContinueOnError)
		flag.VisitAll(func(f *flag.Flag) {
			flagset.Var(f.Value, f.Name, f.Usage)
		})
		flagset.SetOutput(ioutil.Discard)
		strict = false
	}

	if help {
		return flag.ErrHelp
	}
	return nil
}

// Parse is a wrapper around flag.CommandLine.Parse() that is command
// and operation aware. It strives to parse flags that appear either
// before or after the opeeration name.  It supports a
// pseudo-operation "help" which behaves as if the "-h" flag is
// present.
func Parse() error {
	return Command.Parse()
}

func (c *command) Parse() error {
	err := parse()
	if errors.Is(err, flag.ErrHelp) {
		arg := flag.Args()
		if len(arg) > 0 && arg[0] == "help" {
			arg = arg[1:]
		}
		if len(arg) > 0 {
			// if invoked as "COMMAND help OPERATION", show the operation-specific usage
			op, ok := c.operation[arg[0]]
			if ok {
				flag.Usage = op.usage
			}
		}
	}
	if err != nil {
		return err
	}

	if cpuProfileFlag != nil && *cpuProfileFlag != "" {
		f, err := os.Create(*cpuProfileFlag)
		if err != nil {
			return err
		}
		pprof.StartCPUProfile(f) // stopped in Exit()
	}

	return err
}

// Helper function exits, after showing usage, if error is not nil.
func CheckUsage(err error) {
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			status = 2
			fmt.Println(err)
		}

		flag.Usage()

		Exit()
	}
}

// Checkf exits on non-nil error, logging a formatted message.
func Checkf(err error, format string, arg ...interface{}) {
	if err != nil {
		Check(fmt.Errorf(format, arg...))
	}
}

