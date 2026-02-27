package cmd

import (
	"context"
	"fmt"

	"github.com/space-code/linkctl/internal/build"
	"github.com/space-code/linkctl/pkg/cmd/factory"
	"github.com/space-code/linkctl/pkg/cmd/root"
)

type exitCode int

const (
	exitOK    exitCode = 0
	exitError exitCode = 1
)

func Main() exitCode {
	buildVersion := build.Version

	cmdFactory := factory.New(buildVersion)
	stderr := cmdFactory.IOStreams.ErrOut

	ctx := context.Background()

	rootCmd, err := root.NewCmdRoot(cmdFactory, buildVersion)
	if err != nil {
		fmt.Fprintf(stderr, "failed to create root command: %s\n", err)
		return exitError
	}

	if _, err := rootCmd.ExecuteContextC(ctx); err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return exitError
	}

	return exitOK
}
