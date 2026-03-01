package core

import (
	"regexp"
	"testing"
)

func TestNewChatSessionID(t *testing.T) {
	id := NewChatSessionID()
	pat := regexp.MustCompile(`^chat-\d{8}-[0-9a-f]{8}$`)
	if !pat.MatchString(id) {
		t.Fatalf("invalid chat session id: %s", id)
	}
}

func TestNewTaskPlanID(t *testing.T) {
	id := NewTaskPlanID()
	pat := regexp.MustCompile(`^plan-\d{8}-[0-9a-f]{8}$`)
	if !pat.MatchString(id) {
		t.Fatalf("invalid task plan id: %s", id)
	}
}

func TestNewTaskItemID(t *testing.T) {
	id := NewTaskItemID("plan-20260301-a3f1b2c0", 1)
	if id != "task-a3f1b2c0-1" {
		t.Fatalf("unexpected task item id: %s", id)
	}
}

func TestTaskItemValidate(t *testing.T) {
	err := (TaskItem{Description: "   "}).Validate()
	if err == nil {
		t.Fatal("expected validation error for empty description")
	}
}
