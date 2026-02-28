package runtimeprocess

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/user/ai-workflow/internal/core"
)

type ProcessRuntime struct {
	counter  atomic.Int64
	mu       sync.Mutex
	commands map[string]*exec.Cmd
}

func New() *ProcessRuntime {
	return &ProcessRuntime{
		commands: make(map[string]*exec.Cmd),
	}
}

func (r *ProcessRuntime) Name() string {
	return "process"
}

func (r *ProcessRuntime) Init(_ context.Context) error {
	return nil
}

func (r *ProcessRuntime) Close() error {
	return nil
}

func (r *ProcessRuntime) Create(ctx context.Context, opts core.RuntimeOpts) (*core.Session, error) {
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, opts.Command[0], opts.Command[1:]...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	if len(opts.Env) > 0 {
		env := os.Environ()
		for k, v := range opts.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %v: %w", opts.Command, err)
	}

	id := fmt.Sprintf("proc-%d", r.counter.Add(1))
	r.mu.Lock()
	r.commands[id] = cmd
	r.mu.Unlock()

	return &core.Session{
		ID:     id,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Wait: func() error {
			err := cmd.Wait()
			r.mu.Lock()
			delete(r.commands, id)
			r.mu.Unlock()
			return err
		},
	}, nil
}

func (r *ProcessRuntime) Kill(sessionID string) error {
	r.mu.Lock()
	cmd, ok := r.commands[sessionID]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return cmd.Process.Kill()
}
