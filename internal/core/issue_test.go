package core

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewIssueID(t *testing.T) {
	id := NewIssueID()
	pat := regexp.MustCompile(`^issue-\d{8}-[0-9a-f]{8}$`)
	if !pat.MatchString(id) {
		t.Fatalf("invalid issue id: %s", id)
	}
}

func TestIssueValidate_TitleRequired(t *testing.T) {
	issue := Issue{
		Title:    "   ",
		Template: "default",
	}

	err := issue.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty title")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Fatalf("expected title validation error, got: %v", err)
	}
}

func TestIssueValidate_TemplateValidation(t *testing.T) {
	cases := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{name: "empty", template: "", wantErr: true},
		{name: "whitespace", template: "   ", wantErr: true},
		{name: "contains space", template: "foo bar", wantErr: true},
		{name: "valid", template: "default_v1", wantErr: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			issue := Issue{
				Title:    "test issue",
				Template: tc.template,
			}

			err := issue.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected template validation error for template %q", tc.template)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected validation success for template %q, got: %v", tc.template, err)
			}
		})
	}
}
