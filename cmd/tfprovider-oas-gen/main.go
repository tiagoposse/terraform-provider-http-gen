package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-colorable"
	"github.com/mitchellh/cli"
)

func main() {
	name := "tfprovider-oas-gen"
	versionOutput := fmt.Sprintf("%s %s", name, "0.1.0")

	os.Exit(runCLI(
		name,
		versionOutput,
		os.Args[1:],
		os.Stdin,
		colorable.NewColorableStdout(),
		colorable.NewColorableStderr(),
	))
}

func runCLI(name, versionOutput string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ui := &cli.ColoredUi{
		ErrorColor: cli.UiColorRed,
		WarnColor:  cli.UiColorYellow,

		Ui: &cli.BasicUi{
			Reader:      stdin,
			Writer:      stdout,
			ErrorWriter: stderr,
		},
	}

	commands := initCommands(ui)
	frameworkGen := cli.CLI{
		Name:       name,
		Args:       args,
		Commands:   commands,
		HelpFunc:   cli.BasicHelpFunc(name),
		HelpWriter: stderr,
		Version:    versionOutput,
	}
	exitCode, err := frameworkGen.Run()
	if err != nil {
		return 1
	}

	return exitCode
}

func initCommands(ui cli.Ui) map[string]cli.CommandFactory {
	return map[string]cli.CommandFactory{
		// Code generation commands
		"generate": commandFactory(&GenerateAllCommand{UI: ui}),
	}
}

func commandFactory(cmd cli.Command) cli.CommandFactory {
	return func() (cli.Command, error) {
		return cmd, nil
	}
}
