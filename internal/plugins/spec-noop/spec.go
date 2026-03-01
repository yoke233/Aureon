package specnoop

import (
	"context"

	"github.com/user/ai-workflow/internal/core"
)

// NoopSpec is a fallback provider that returns empty spec context.
type NoopSpec struct{}

func New() *NoopSpec {
	return &NoopSpec{}
}

func (n *NoopSpec) Name() string {
	return "spec-noop"
}

func (n *NoopSpec) Init(context.Context) error {
	return nil
}

func (n *NoopSpec) Close() error {
	return nil
}

func (n *NoopSpec) IsInitialized() bool {
	return false
}

func (n *NoopSpec) GetContext(context.Context, core.SpecContextRequest) (core.SpecContext, error) {
	return core.SpecContext{}, nil
}

var _ core.SpecPlugin = (*NoopSpec)(nil)
