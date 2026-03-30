package main

import (
	"strconv"

	"github.com/spf13/cobra"
)

func newOrchestrateCmd(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orchestrate",
		Short: "Run orchestration control actions",
	}
	cmd.AddCommand(newOrchestrateTaskCmd(deps))
	return cmd
}

func newOrchestrateTaskCmd(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Run task orchestration actions",
	}
	cmd.AddCommand(newOrchestrateTaskCreateCmd(deps))
	return cmd
}

func newOrchestrateTaskCreateCmd(deps commandDeps) *cobra.Command {
	var title string
	var projectID int64
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an orchestrated task",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			forwardArgs := []string{"task", "create"}
			if cmd.Flags().Changed("title") {
				forwardArgs = append(forwardArgs, "--title", title)
			}
			if cmd.Flags().Changed("project-id") {
				forwardArgs = append(forwardArgs, "--project-id", strconv.FormatInt(projectID, 10))
			}
			if cmd.Flags().Changed("json") && jsonOutput {
				forwardArgs = append(forwardArgs, "--json")
			}
			return deps.runOrchestrate(forwardArgs)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Task title")
	cmd.Flags().Int64Var(&projectID, "project-id", 0, "Project ID")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit JSON output")
	return cmd
}
