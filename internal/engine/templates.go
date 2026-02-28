package engine

import "github.com/user/ai-workflow/internal/core"

var Templates = map[string][]core.StageID{
	"full": {
		core.StageRequirements, core.StageSpecGen, core.StageSpecReview,
		core.StageWorktreeSetup, core.StageImplement, core.StageCodeReview,
		core.StageFixup, core.StageMerge, core.StageCleanup,
	},
	"standard": {
		core.StageRequirements, core.StageWorktreeSetup, core.StageImplement,
		core.StageCodeReview, core.StageFixup, core.StageMerge, core.StageCleanup,
	},
	"quick": {
		core.StageRequirements, core.StageWorktreeSetup, core.StageImplement,
		core.StageCodeReview, core.StageMerge, core.StageCleanup,
	},
	"hotfix": {
		core.StageWorktreeSetup, core.StageImplement, core.StageMerge, core.StageCleanup,
	},
}
