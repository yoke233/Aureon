package builtin

import "embed"

//go:embed step-signal
var StepSignalFS embed.FS

//go:embed sys-step-manage
var SysStepManageFS embed.FS

//go:embed track-planning
var TrackPlanningFS embed.FS

//go:embed all:*
var AllBuiltinFS embed.FS
