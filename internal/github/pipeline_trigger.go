package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/engine"
)

const defaultPipelineTemplate = "standard"

type pipelineCreateFn func(projectID, name, description, template string) (*core.Pipeline, error)

// PipelineTrigger creates pipelines from issue and slash command events.
type PipelineTrigger struct {
	store          core.Store
	createPipeline pipelineCreateFn
	now            func() time.Time
}

type IssueTriggerInput struct {
	ProjectID            string
	IssueNumber          int
	IssueTitle           string
	IssueBody            string
	Labels               []string
	LabelTemplateMapping map[string]string
	TraceID              string
}

type CommandTriggerInput struct {
	ProjectID            string
	IssueNumber          int
	Message              string
	Template             string
	DefaultTemplate      string
	LabelTemplateMapping map[string]string
	Labels               []string
	TraceID              string
}

func NewPipelineTrigger(store core.Store, create pipelineCreateFn) *PipelineTrigger {
	return &PipelineTrigger{
		store:          store,
		createPipeline: create,
		now:            time.Now,
	}
}

func (t *PipelineTrigger) TriggerFromIssue(ctx context.Context, input IssueTriggerInput) (*core.Pipeline, error) {
	template := pickTemplate(input.Labels, input.LabelTemplateMapping, defaultPipelineTemplate)
	name := strings.TrimSpace(input.IssueTitle)
	if name == "" {
		name = fmt.Sprintf("Issue #%d", input.IssueNumber)
	}
	return t.trigger(ctx, triggerInput{
		ProjectID:   input.ProjectID,
		IssueNumber: input.IssueNumber,
		Name:        name,
		Description: input.IssueBody,
		Template:    template,
		TraceID:     input.TraceID,
		Source:      "issue_opened",
	})
}

func (t *PipelineTrigger) TriggerFromCommand(ctx context.Context, input CommandTriggerInput) (*core.Pipeline, error) {
	template := strings.TrimSpace(input.Template)
	if template == "" {
		defaultTemplate := input.DefaultTemplate
		if strings.TrimSpace(defaultTemplate) == "" {
			defaultTemplate = defaultPipelineTemplate
		}
		template = pickTemplate(input.Labels, input.LabelTemplateMapping, defaultTemplate)
	}
	description := strings.TrimSpace(input.Message)
	if description == "" {
		description = "triggered by slash command"
	}

	return t.trigger(ctx, triggerInput{
		ProjectID:   input.ProjectID,
		IssueNumber: input.IssueNumber,
		Name:        fmt.Sprintf("Issue #%d command run", input.IssueNumber),
		Description: description,
		Template:    template,
		TraceID:     input.TraceID,
		Source:      "slash_run",
	})
}

type triggerInput struct {
	ProjectID   string
	IssueNumber int
	Name        string
	Description string
	Template    string
	TraceID     string
	Source      string
}

func (t *PipelineTrigger) trigger(ctx context.Context, input triggerInput) (*core.Pipeline, error) {
	if t == nil || t.store == nil {
		return nil, errors.New("pipeline trigger store is required")
	}
	if t.createPipeline == nil {
		return nil, errors.New("pipeline trigger create function is required")
	}
	projectID := strings.TrimSpace(input.ProjectID)
	if projectID == "" {
		return nil, errors.New("project id is required")
	}
	if input.IssueNumber <= 0 {
		return nil, errors.New("issue number must be positive")
	}

	existing, err := t.findExistingIssuePipeline(projectID, input.IssueNumber)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	template := strings.TrimSpace(input.Template)
	if template == "" {
		template = defaultPipelineTemplate
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = fmt.Sprintf("Issue #%d", input.IssueNumber)
	}
	pipeline, err := t.createPipeline(projectID, name, input.Description, template)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		return nil, errors.New("create pipeline returned nil")
	}
	if pipeline.Config == nil {
		pipeline.Config = map[string]any{}
	}
	pipeline.Config["issue_number"] = input.IssueNumber
	pipeline.Config["trigger_source"] = input.Source
	if traceID := strings.TrimSpace(input.TraceID); traceID != "" {
		pipeline.Config["trace_id"] = traceID
	}
	if pipeline.QueuedAt.IsZero() {
		pipeline.QueuedAt = t.now()
	}
	if pipeline.CreatedAt.IsZero() {
		pipeline.CreatedAt = t.now()
	}
	pipeline.UpdatedAt = t.now()

	if err := t.store.SavePipeline(pipeline); err != nil {
		return nil, err
	}
	return pipeline, nil
}

func (t *PipelineTrigger) findExistingIssuePipeline(projectID string, issueNumber int) (*core.Pipeline, error) {
	return engine.FindPipelineByIssueNumber(t.store, projectID, issueNumber)
}

func pickTemplate(labels []string, mapping map[string]string, fallback string) string {
	if len(mapping) == 0 {
		if strings.TrimSpace(fallback) == "" {
			return defaultPipelineTemplate
		}
		return strings.TrimSpace(fallback)
	}
	for _, label := range labels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if normalized == "" {
			continue
		}
		for pattern, template := range mapping {
			if strings.EqualFold(strings.TrimSpace(pattern), normalized) {
				if trimmed := strings.TrimSpace(template); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	if strings.TrimSpace(fallback) == "" {
		return defaultPipelineTemplate
	}
	return strings.TrimSpace(fallback)
}
