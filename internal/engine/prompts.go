package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type PromptVars struct {
	ProjectName    string
	ChangeName     string
	RepoPath       string
	WorktreePath   string
	Requirements   string
	SpecPath       string
	TasksMD        string
	PreviousReview string
	HumanFeedback  string
	RetryError     string
	RetryCount     int
}

func RenderPrompt(stage string, vars PromptVars) (string, error) {
	path := filepath.Join("configs", "prompts", stage+".tmpl")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Execute stage: %s\nRequirements: %s", stage, vars.Requirements), nil
	}

	tmpl, err := template.New(stage).Parse(string(data))
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, vars); err != nil {
		return "", err
	}
	return b.String(), nil
}
