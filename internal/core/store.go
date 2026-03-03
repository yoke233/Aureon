package core

type ProjectFilter struct {
	NameContains string
}

type PipelineFilter struct {
	Status string
	Limit  int
	Offset int
}

type IssueFilter struct {
	Status    string
	SessionID string
	State     string
	Limit     int
	Offset    int
}

type IssueAttachment struct {
	ID        string `json:"id"`
	IssueID   string `json:"issue_id"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type IssueChange struct {
	ID        string `json:"id"`
	IssueID   string `json:"issue_id"`
	Field     string `json:"field"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
	Reason    string `json:"reason"`
	ChangedBy string `json:"changed_by"`
	CreatedAt string `json:"created_at"`
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

	CreateIssue(i *Issue) error
	GetIssue(id string) (*Issue, error)
	SaveIssue(i *Issue) error
	ListIssues(projectID string, filter IssueFilter) ([]Issue, int, error)
	GetActiveIssues(projectID string) ([]Issue, error)
	GetIssueByPipeline(pipelineID string) (*Issue, error)
	SaveIssueAttachment(issueID, path, content string) error
	GetIssueAttachments(issueID string) ([]IssueAttachment, error)
	SaveIssueChange(change *IssueChange) error
	GetIssueChanges(issueID string) ([]IssueChange, error)

	SaveReviewRecord(r *ReviewRecord) error
	GetReviewRecords(issueID string) ([]ReviewRecord, error)

	Close() error
}
