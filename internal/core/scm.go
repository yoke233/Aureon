package core

import "context"

// SCM defines source-control operations used by pipeline/task automation.
type SCM interface {
	Plugin
	CreateBranch(ctx context.Context, branch string) error
	Commit(ctx context.Context, message string) (commitHash string, err error)
	Push(ctx context.Context, remote string, branch string) error
	Merge(ctx context.Context, branch string) (mergeCommit string, err error)
	CreatePR(ctx context.Context, req PullRequest) (prURL string, err error)
}

type PullRequest struct {
	Title string
	Body  string
	Head  string
	Base  string
}
