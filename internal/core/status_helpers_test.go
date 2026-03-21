package core

import "testing"

func TestActionTypeCompositeExists(t *testing.T) {
	if ActionComposite != "composite" {
		t.Fatalf("ActionComposite = %q, want %q", ActionComposite, "composite")
	}
}

func TestActionTypeValid(t *testing.T) {
	valid := []ActionType{ActionExec, ActionGate, ActionPlan, ActionComposite}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("ActionType(%q).Valid() = false, want true", v)
		}
	}
	if ActionType("bogus").Valid() {
		t.Error("ActionType(bogus).Valid() = true, want false")
	}
}

func TestParseActionType(t *testing.T) {
	got, err := ParseActionType("exec")
	if err != nil || got != ActionExec {
		t.Fatalf("ParseActionType(exec) = (%q, %v), want (exec, nil)", got, err)
	}
	_, err = ParseActionType("invalid")
	if err == nil {
		t.Fatal("ParseActionType(invalid) should return error")
	}
}

func TestActionStatusValid(t *testing.T) {
	valid := []ActionStatus{ActionPending, ActionReady, ActionRunning, ActionWaitingGate, ActionBlocked, ActionFailed, ActionDone, ActionCancelled}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("ActionStatus(%q).Valid() = false, want true", v)
		}
	}
	if ActionStatus("bogus").Valid() {
		t.Error("ActionStatus(bogus).Valid() = true, want false")
	}
}

func TestParseActionStatus(t *testing.T) {
	got, err := ParseActionStatus("running")
	if err != nil || got != ActionRunning {
		t.Fatalf("ParseActionStatus(running) = (%q, %v), want (running, nil)", got, err)
	}
	_, err = ParseActionStatus("nope")
	if err == nil {
		t.Fatal("ParseActionStatus(nope) should return error")
	}
}

func TestRunStatusValid(t *testing.T) {
	valid := []RunStatus{RunCreated, RunRunning, RunSucceeded, RunFailed, RunCancelled}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("RunStatus(%q).Valid() = false, want true", v)
		}
	}
	if RunStatus("bogus").Valid() {
		t.Error("RunStatus(bogus).Valid() = true, want false")
	}
}

func TestParseRunStatus(t *testing.T) {
	got, err := ParseRunStatus("running")
	if err != nil || got != RunRunning {
		t.Fatalf("ParseRunStatus(running) = (%q, %v), want (running, nil)", got, err)
	}
	_, err = ParseRunStatus("nope")
	if err == nil {
		t.Fatal("ParseRunStatus(nope) should return error")
	}
}

func TestWorkItemStatusValid(t *testing.T) {
	valid := []WorkItemStatus{
		WorkItemOpen, WorkItemAccepted, WorkItemQueued, WorkItemRunning,
		WorkItemBlocked, WorkItemFailed, WorkItemDone, WorkItemCancelled, WorkItemClosed,
	}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("WorkItemStatus(%q).Valid() = false, want true", v)
		}
	}
	if WorkItemStatus("bogus").Valid() {
		t.Error("WorkItemStatus(bogus).Valid() = true, want false")
	}
}

func TestParseWorkItemStatus(t *testing.T) {
	got, err := ParseWorkItemStatus("running")
	if err != nil || got != WorkItemRunning {
		t.Fatalf("ParseWorkItemStatus(running) = (%q, %v), want (running, nil)", got, err)
	}
	_, err = ParseWorkItemStatus("nope")
	if err == nil {
		t.Fatal("ParseWorkItemStatus(nope) should return error")
	}
}

func TestCanTransitionWorkItemStatus(t *testing.T) {
	if !CanTransitionWorkItemStatus(WorkItemOpen, WorkItemAccepted) {
		t.Error("open -> accepted should be valid")
	}
	if CanTransitionWorkItemStatus(WorkItemDone, WorkItemOpen) {
		t.Error("done -> open should be invalid")
	}
	if !CanTransitionWorkItemStatus(WorkItemRunning, WorkItemRunning) {
		t.Error("same status transition should be valid")
	}
}

func TestFeatureStatusValid(t *testing.T) {
	valid := []FeatureStatus{FeaturePending, FeaturePass, FeatureFail, FeatureSkipped}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("FeatureStatus(%q).Valid() = false, want true", v)
		}
	}
	if FeatureStatus("bogus").Valid() {
		t.Error("FeatureStatus(bogus).Valid() = true, want false")
	}
}

func TestParseFeatureStatus(t *testing.T) {
	got, err := ParseFeatureStatus("pass")
	if err != nil || got != FeaturePass {
		t.Fatalf("ParseFeatureStatus(pass) = (%q, %v), want (pass, nil)", got, err)
	}
	_, err = ParseFeatureStatus("nope")
	if err == nil {
		t.Fatal("ParseFeatureStatus(nope) should return error")
	}
}

func TestInspectionStatusValid(t *testing.T) {
	valid := []InspectionStatus{InspectionStatusPending, InspectionStatusRunning, InspectionStatusCompleted, InspectionStatusFailed}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("InspectionStatus(%q).Valid() = false, want true", v)
		}
	}
	if InspectionStatus("bogus").Valid() {
		t.Error("InspectionStatus(bogus).Valid() = true, want false")
	}
}

func TestParseInspectionStatus(t *testing.T) {
	got, err := ParseInspectionStatus("running")
	if err != nil || got != InspectionStatusRunning {
		t.Fatalf("ParseInspectionStatus(running) = (%q, %v), want (running, nil)", got, err)
	}
	_, err = ParseInspectionStatus("nope")
	if err == nil {
		t.Fatal("ParseInspectionStatus(nope) should return error")
	}
}

func TestRunProbeStatusValid(t *testing.T) {
	valid := []RunProbeStatus{RunProbePending, RunProbeSent, RunProbeAnswered, RunProbeTimeout, RunProbeUnreachable, RunProbeFailed}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("RunProbeStatus(%q).Valid() = false, want true", v)
		}
	}
	if RunProbeStatus("bogus").Valid() {
		t.Error("RunProbeStatus(bogus).Valid() = true, want false")
	}
}

func TestParseRunProbeStatus(t *testing.T) {
	got, err := ParseRunProbeStatus("sent")
	if err != nil || got != RunProbeSent {
		t.Fatalf("ParseRunProbeStatus(sent) = (%q, %v), want (sent, nil)", got, err)
	}
	_, err = ParseRunProbeStatus("nope")
	if err == nil {
		t.Fatal("ParseRunProbeStatus(nope) should return error")
	}
}

func TestRunProbeVerdictValid(t *testing.T) {
	valid := []RunProbeVerdict{RunProbeAlive, RunProbeBlocked, RunProbeHung, RunProbeDead, RunProbeUnknown}
	for _, v := range valid {
		if !v.Valid() {
			t.Errorf("RunProbeVerdict(%q).Valid() = false, want true", v)
		}
	}
	if RunProbeVerdict("bogus").Valid() {
		t.Error("RunProbeVerdict(bogus).Valid() = true, want false")
	}
}

func TestParseInitiativeStatus(t *testing.T) {
	got, err := ParseInitiativeStatus("executing")
	if err != nil || got != InitiativeExecuting {
		t.Fatalf("ParseInitiativeStatus(executing) = (%q, %v), want (executing, nil)", got, err)
	}
	_, err = ParseInitiativeStatus("nope")
	if err == nil {
		t.Fatal("ParseInitiativeStatus(nope) should return error")
	}
}
