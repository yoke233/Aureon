package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/eventbus"
)

type Executor struct {
	store   core.Store
	bus     *eventbus.Bus
	agents  map[string]core.AgentPlugin
	runtime core.RuntimePlugin
	logger  *slog.Logger
}

func NewExecutor(
	store core.Store,
	bus *eventbus.Bus,
	agents map[string]core.AgentPlugin,
	runtime core.RuntimePlugin,
	logger *slog.Logger,
) *Executor {
	return &Executor{
		store:   store,
		bus:     bus,
		agents:  agents,
		runtime: runtime,
		logger:  logger,
	}
}

func (e *Executor) CreatePipeline(projectID, name, description, template string) (*core.Pipeline, error) {
	stageIDs, ok := Templates[template]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s", template)
	}

	stages := make([]core.StageConfig, len(stageIDs))
	for i, sid := range stageIDs {
		stages[i] = defaultStageConfig(sid)
	}

	p := &core.Pipeline{
		ID:              NewPipelineID(),
		ProjectID:       projectID,
		Name:            name,
		Description:     description,
		Template:        template,
		Status:          core.StatusCreated,
		Stages:          stages,
		Artifacts:       map[string]string{},
		MaxTotalRetries: 5,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := e.store.SavePipeline(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (e *Executor) Run(ctx context.Context, pipelineID string) error {
	p, err := e.store.GetPipeline(pipelineID)
	if err != nil {
		return err
	}

	if err := core.ValidateTransition(p.Status, core.StatusRunning); err != nil {
		return err
	}
	p.Status = core.StatusRunning
	p.StartedAt = time.Now()
	if err := e.store.SavePipeline(p); err != nil {
		return err
	}

	for i, stage := range p.Stages {
		p.CurrentStage = stage.Name
		if err := e.store.SavePipeline(p); err != nil {
			return err
		}

		e.bus.Publish(core.Event{
			Type:       core.EventStageStart,
			PipelineID: p.ID,
			ProjectID:  p.ProjectID,
			Stage:      stage.Name,
			Timestamp:  time.Now(),
		})

		cp := &core.Checkpoint{
			PipelineID: p.ID,
			StageName:  stage.Name,
			Status:     core.CheckpointInProgress,
			StartedAt:  time.Now(),
			AgentUsed:  stage.Agent,
		}
		if err := e.store.SaveCheckpoint(cp); err != nil {
			return err
		}

		err := e.executeStage(ctx, p, &p.Stages[i])
		cp.FinishedAt = time.Now()

		if err != nil {
			cp.Status = core.CheckpointFailed
			cp.Error = err.Error()
			if saveErr := e.store.SaveCheckpoint(cp); saveErr != nil {
				return saveErr
			}

			e.bus.Publish(core.Event{
				Type:       core.EventStageFailed,
				PipelineID: p.ID,
				Stage:      stage.Name,
				Error:      err.Error(),
				Timestamp:  time.Now(),
			})

			p.TotalRetries++
			if p.TotalRetries >= p.MaxTotalRetries {
				p.Status = core.StatusFailed
				p.ErrorMessage = fmt.Sprintf("retry budget exhausted at stage %s: %v", stage.Name, err)
				p.FinishedAt = time.Now()
				if saveErr := e.store.SavePipeline(p); saveErr != nil {
					return saveErr
				}
				e.bus.Publish(core.Event{
					Type:       core.EventPipelineFailed,
					PipelineID: p.ID,
					Timestamp:  time.Now(),
				})
				return fmt.Errorf("pipeline failed: %w", err)
			}

			if stage.OnFailure == core.OnFailureHuman {
				p.Status = core.StatusWaitingHuman
				if saveErr := e.store.SavePipeline(p); saveErr != nil {
					return saveErr
				}
				e.bus.Publish(core.Event{
					Type:       core.EventHumanRequired,
					PipelineID: p.ID,
					Stage:      stage.Name,
					Timestamp:  time.Now(),
				})
				return nil
			}

			if stage.OnFailure == core.OnFailureAbort {
				p.Status = core.StatusFailed
				p.FinishedAt = time.Now()
				if saveErr := e.store.SavePipeline(p); saveErr != nil {
					return saveErr
				}
				return fmt.Errorf("stage %s failed, aborting: %w", stage.Name, err)
			}

			continue
		}

		cp.Status = core.CheckpointSuccess
		if err := e.store.SaveCheckpoint(cp); err != nil {
			return err
		}

		e.bus.Publish(core.Event{
			Type:       core.EventStageComplete,
			PipelineID: p.ID,
			Stage:      stage.Name,
			Timestamp:  time.Now(),
		})

		if stage.RequireHuman {
			p.Status = core.StatusWaitingHuman
			if err := e.store.SavePipeline(p); err != nil {
				return err
			}
			e.bus.Publish(core.Event{
				Type:       core.EventHumanRequired,
				PipelineID: p.ID,
				Stage:      stage.Name,
				Timestamp:  time.Now(),
			})
			return nil
		}
	}

	p.Status = core.StatusDone
	p.FinishedAt = time.Now()
	if err := e.store.SavePipeline(p); err != nil {
		return err
	}
	e.bus.Publish(core.Event{
		Type:       core.EventPipelineDone,
		PipelineID: p.ID,
		Timestamp:  time.Now(),
	})
	return nil
}

func (e *Executor) executeStage(ctx context.Context, p *core.Pipeline, stage *core.StageConfig) error {
	switch stage.Name {
	case core.StageWorktreeSetup, core.StageMerge, core.StageCleanup:
		if e.logger != nil {
			e.logger.Info("executing built-in stage", "stage", stage.Name, "pipeline", p.ID)
		}
		return nil
	}

	agent, ok := e.agents[stage.Agent]
	if !ok {
		return fmt.Errorf("agent %q not found", stage.Agent)
	}

	opts := core.ExecOpts{
		Prompt:   fmt.Sprintf("Execute stage: %s\nDescription: %s", stage.Name, p.Description),
		WorkDir:  p.WorktreePath,
		MaxTurns: 30,
		Timeout:  stage.Timeout,
	}
	cmd, err := agent.BuildCommand(opts)
	if err != nil {
		return fmt.Errorf("build command: %w", err)
	}

	stageCtx := ctx
	if stage.Timeout > 0 {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, stage.Timeout)
		defer cancel()
	}

	sess, err := e.runtime.Create(stageCtx, core.RuntimeOpts{
		Command: cmd,
		WorkDir: p.WorktreePath,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	parser := agent.NewStreamParser(sess.Stdout)
	for {
		evt, err := parser.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
		e.bus.Publish(core.Event{
			Type:       core.EventAgentOutput,
			PipelineID: p.ID,
			Stage:      stage.Name,
			Agent:      stage.Agent,
			Data: map[string]string{
				"content": evt.Content,
				"type":    evt.Type,
			},
			Timestamp: evt.Timestamp,
		})
	}

	return sess.Wait()
}

func defaultStageConfig(id core.StageID) core.StageConfig {
	cfg := core.StageConfig{
		Name:       id,
		Timeout:    30 * time.Minute,
		MaxRetries: 1,
		OnFailure:  core.OnFailureHuman,
	}
	switch id {
	case core.StageRequirements, core.StageSpecGen, core.StageSpecReview, core.StageCodeReview:
		cfg.Agent = "claude"
	case core.StageImplement, core.StageFixup:
		cfg.Agent = "codex"
	case core.StageWorktreeSetup, core.StageMerge, core.StageCleanup:
		cfg.Agent = ""
		cfg.Timeout = 2 * time.Minute
	}
	cfg.PromptTemplate = string(id)
	return cfg
}
