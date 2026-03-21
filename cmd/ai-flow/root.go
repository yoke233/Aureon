package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/yoke233/zhanggui/internal/platform/appcmd"
)

const versionString = "ai-flow v0.1.0-dev"

type commandDeps struct {
	out            io.Writer
	err            io.Writer
	version        string
	runServer      func([]string) error
	runExecutor    func([]string) error
	runQualityGate func([]string) error
	runMCPServe    func([]string) error
}

func defaultCommandDeps() commandDeps {
	return commandDeps{
		out:            os.Stdout,
		err:            os.Stderr,
		version:        versionString,
		runServer:      appcmd.RunServer,
		runExecutor:    appcmd.RunExecutor,
		runQualityGate: appcmd.RunQualityGate,
		runMCPServe:    appcmd.RunMCPServe,
	}
}

func newRootCmd(deps commandDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = io.Discard
	}
	if deps.err == nil {
		deps.err = io.Discard
	}
	if deps.version == "" {
		deps.version = versionString
	}

	rootCmd := &cobra.Command{
		Use:           "ai-flow",
		Short:         "AI Workflow Orchestrator",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.SetOut(deps.out)
	rootCmd.SetErr(deps.err)
	rootCmd.AddCommand(
		newVersionCmd(deps),
		newServerCmd(deps),
		newExecutorCmd(deps),
		newQualityGateCmd(deps),
		newMCPServeCmd(deps),
	)
	return rootCmd
}

func newVersionCmd(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), deps.version)
			return err
		},
	}
}

func newServerCmd(deps commandDeps) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the HTTP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			forwardArgs := make([]string, 0, 2)
			if cmd.Flags().Changed("port") {
				forwardArgs = append(forwardArgs, "--port", strconv.Itoa(port))
			}
			return deps.runServer(forwardArgs)
		},
	}
	cmd.Flags().IntVar(&port, "port", 0, "Listen port")
	return cmd
}

func newExecutorCmd(deps commandDeps) *cobra.Command {
	var natsURL string
	var agents string
	var maxConcurrent int
	cmd := &cobra.Command{
		Use:   "executor",
		Short: "Run the executor worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			forwardArgs := make([]string, 0, 6)
			if cmd.Flags().Changed("nats-url") {
				forwardArgs = append(forwardArgs, "--nats-url", natsURL)
			}
			if cmd.Flags().Changed("agents") {
				forwardArgs = append(forwardArgs, "--agents", agents)
			}
			if cmd.Flags().Changed("max-concurrent") {
				forwardArgs = append(forwardArgs, "--max-concurrent", strconv.Itoa(maxConcurrent))
			}
			return deps.runExecutor(forwardArgs)
		},
	}
	cmd.Flags().StringVar(&natsURL, "nats-url", "", "NATS server URL")
	cmd.Flags().StringVar(&agents, "agents", "", "Comma-separated agent types")
	cmd.Flags().IntVar(&maxConcurrent, "max-concurrent", 2, "Maximum concurrent executions")
	return cmd
}

func newQualityGateCmd(deps commandDeps) *cobra.Command {
	var backendOnly bool
	var frontendOnly bool
	var skipBackend bool
	var skipFrontend bool
	cmd := &cobra.Command{
		Use:   "quality-gate",
		Short: "Run backend and frontend quality checks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			forwardArgs := make([]string, 0, 4)
			if cmd.Flags().Changed("backend-only") && backendOnly {
				forwardArgs = append(forwardArgs, "--backend-only")
			}
			if cmd.Flags().Changed("frontend-only") && frontendOnly {
				forwardArgs = append(forwardArgs, "--frontend-only")
			}
			if cmd.Flags().Changed("skip-backend") && skipBackend {
				forwardArgs = append(forwardArgs, "--skip-backend")
			}
			if cmd.Flags().Changed("skip-frontend") && skipFrontend {
				forwardArgs = append(forwardArgs, "--skip-frontend")
			}
			return deps.runQualityGate(forwardArgs)
		},
	}
	cmd.Flags().BoolVar(&backendOnly, "backend-only", false, "Run only backend checks")
	cmd.Flags().BoolVar(&frontendOnly, "frontend-only", false, "Run only frontend checks")
	cmd.Flags().BoolVar(&skipBackend, "skip-backend", false, "Skip backend checks")
	cmd.Flags().BoolVar(&skipFrontend, "skip-frontend", false, "Skip frontend checks")
	return cmd
}

func newMCPServeCmd(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp-serve",
		Short: "Run the stdio MCP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return deps.runMCPServe(nil)
		},
	}
}
