package specnoop

import "github.com/user/ai-workflow/internal/core"

func Module() core.PluginModule {
	return core.PluginModule{
		Name: "noop",
		Slot: core.SlotSpec,
		Factory: func(map[string]any) (core.Plugin, error) {
			return New(), nil
		},
	}
}
