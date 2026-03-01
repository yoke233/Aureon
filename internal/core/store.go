package core

type ProjectFilter struct {
	NameContains string
}

type PipelineFilter struct {
	Status string
	Limit  int
	Offset int
}

type TaskPlanFilter struct {
	Status string
	Limit  int
	Offset int
}

type LogEntry struct {
	ID         int64  `json:"id"`
	PipelineID string `json:"pipeline_id"`
	Stage      string `json:"stage"`
	Type       string `json:"type"`
	Agent      string `json:"agent"`
	Content    string `json:"content"`
	Timestamp  string `json:"timestamp"`
}

type HumanAction struct {
	ID         int64  `json:"id"`
	PipelineID string `json:"pipeline_id"`
	Stage      string `json:"stage"`
	Action     string `json:"action"`
	Message    string `json:"message"`
	Source     string `json:"source"`
	UserID     string `json:"user_id"`
	CreatedAt  string `json:"created_at"`
}

type Store interface {
	ListProjects(filter ProjectFilter) ([]Project, error)
	GetProject(id string) (*Project, error)
	CreateProject(p *Project) error
	UpdateProject(p *Project) error
	DeleteProject(id string) error

	ListPipelines(projectID string, filter PipelineFilter) ([]Pipeline, error)
	GetPipeline(id string) (*Pipeline, error)
	SavePipeline(p *Pipeline) error
	GetActivePipelines() ([]Pipeline, error)
	ListRunnablePipelines(limit int) ([]Pipeline, error)
	CountRunningPipelinesByProject(projectID string) (int, error)
	TryMarkPipelineRunning(id string, from ...PipelineStatus) (bool, error)

	SaveCheckpoint(cp *Checkpoint) error
	GetCheckpoints(pipelineID string) ([]Checkpoint, error)
	GetLastSuccessCheckpoint(pipelineID string) (*Checkpoint, error)
	InvalidateCheckpointsFromStage(pipelineID string, stage StageID) error

	AppendLog(entry LogEntry) error
	GetLogs(pipelineID string, stage string, limit int, offset int) ([]LogEntry, int, error)

	RecordAction(action HumanAction) error
	GetActions(pipelineID string) ([]HumanAction, error)

	CreateChatSession(s *ChatSession) error
	GetChatSession(id string) (*ChatSession, error)
	UpdateChatSession(s *ChatSession) error
	ListChatSessions(projectID string) ([]ChatSession, error)

	CreateTaskPlan(p *TaskPlan) error
	GetTaskPlan(id string) (*TaskPlan, error)
	SaveTaskPlan(p *TaskPlan) error
	ListTaskPlans(projectID string, filter TaskPlanFilter) ([]TaskPlan, error)
	GetActiveTaskPlans() ([]TaskPlan, error)

	CreateTaskItem(item *TaskItem) error
	GetTaskItem(id string) (*TaskItem, error)
	SaveTaskItem(item *TaskItem) error
	GetTaskItemsByPlan(planID string) ([]TaskItem, error)
	GetTaskItemByPipeline(pipelineID string) (*TaskItem, error)

	SaveReviewRecord(r *ReviewRecord) error
	GetReviewRecords(planID string) ([]ReviewRecord, error)

	Close() error
}
