package main

import (
	"os"
)

func main() {
	if err := run(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	return runWithArgs(os.Args[1:])
}

func runWithArgs(args []string) error {
	cmd := newRootCmd(defaultCommandDeps())
	cmd.SetArgs(args)
	return cmd.Execute()
}
