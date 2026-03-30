package appcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type orchestrateCLIOptions struct {
	Action    string
	Title     string
	ProjectID *int64
	JSON      bool
}

type orchestrateResult struct {
	OK          bool   `json:"ok"`
	Action      string `json:"action"`
	Summary     string `json:"summary"`
	Title       string `json:"title,omitempty"`
	ProjectID   *int64 `json:"project_id,omitempty"`
	Implemented bool   `json:"implemented"`
}

func RunOrchestrate(args []string) error {
	return runOrchestrateToWriter(os.Stdout, args)
}

func runOrchestrateToWriter(out io.Writer, args []string) error {
	opts, err := parseOrchestrateArgs(args)
	if err != nil {
		return err
	}
	if out == nil {
		out = io.Discard
	}
	return writeOrchestrateResult(out, opts)
}

func parseOrchestrateArgs(args []string) (orchestrateCLIOptions, error) {
	if len(args) < 2 {
		return orchestrateCLIOptions{}, fmt.Errorf("usage: ai-flow orchestrate task create [--title <title>] [--project-id <id>] [--json]")
	}
	if strings.TrimSpace(args[0]) != "task" || strings.TrimSpace(args[1]) != "create" {
		return orchestrateCLIOptions{}, fmt.Errorf("usage: ai-flow orchestrate task create [--title <title>] [--project-id <id>] [--json]")
	}

	opts := orchestrateCLIOptions{Action: "task.create"}
	for i := 2; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--title":
			i++
			if i >= len(args) {
				return orchestrateCLIOptions{}, fmt.Errorf("missing value for --title")
			}
			opts.Title = strings.TrimSpace(args[i])
		case strings.HasPrefix(arg, "--title="):
			opts.Title = strings.TrimSpace(strings.TrimPrefix(arg, "--title="))
		case arg == "--project-id":
			i++
			if i >= len(args) {
				return orchestrateCLIOptions{}, fmt.Errorf("missing value for --project-id")
			}
			value, err := parseOrchestrateProjectID(args[i])
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.ProjectID = value
		case strings.HasPrefix(arg, "--project-id="):
			value, err := parseOrchestrateProjectID(strings.TrimPrefix(arg, "--project-id="))
			if err != nil {
				return orchestrateCLIOptions{}, err
			}
			opts.ProjectID = value
		case arg == "--json":
			opts.JSON = true
		default:
			return orchestrateCLIOptions{}, fmt.Errorf("unknown flag: %s", arg)
		}
	}
	return opts, nil
}

func parseOrchestrateProjectID(raw string) (*int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || n <= 0 {
		return nil, fmt.Errorf("invalid value for --project-id: %s", raw)
	}
	return &n, nil
}

func writeOrchestrateResult(out io.Writer, opts orchestrateCLIOptions) error {
	result := orchestrateResult{
		OK:          true,
		Action:      opts.Action,
		Summary:     "orchestration CLI skeleton only; business action not implemented",
		Title:       opts.Title,
		ProjectID:   opts.ProjectID,
		Implemented: false,
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(result)
}
