package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/ai-workflow/internal/core"
	"github.com/user/ai-workflow/internal/tui/views"
)

type Model struct {
	store     core.Store
	pipelines []core.Pipeline
	cursor    int
	err       error
}

func NewModel(store core.Store) Model {
	return Model{store: store}
}

type pipelinesLoadedMsg []core.Pipeline
type errMsg error

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		projects, err := m.store.ListProjects(core.ProjectFilter{})
		if err != nil {
			return errMsg(err)
		}
		var all []core.Pipeline
		for _, proj := range projects {
			pipes, err := m.store.ListPipelines(proj.ID, core.PipelineFilter{})
			if err != nil {
				return errMsg(err)
			}
			all = append(all, pipes...)
		}
		return pipelinesLoadedMsg(all)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pipelinesLoadedMsg:
		m.pipelines = msg
	case errMsg:
		m.err = msg
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.pipelines)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(StyleTitle.Render("AI Workflow Orchestrator") + "\n\n")

	if m.err != nil {
		b.WriteString(fmt.Sprintf("Error: %v\n", m.err))
		return b.String()
	}

	statusRenderer := map[string]func(string) string{}
	for k, st := range StyleStatus {
		style := st
		statusRenderer[k] = func(s string) string {
			return style.Render(s)
		}
	}
	b.WriteString(views.RenderPipelineList(m.pipelines, m.cursor, statusRenderer))
	b.WriteString("\n" + StyleHelp.Render("up/down navigate, q quit"))
	return b.String()
}

func Run(store core.Store) error {
	p := tea.NewProgram(NewModel(store))
	_, err := p.Run()
	return err
}
