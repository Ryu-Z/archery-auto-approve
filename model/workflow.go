package model

import "strings"

type Workflow struct {
	ID           int              `json:"id"`
	WorkflowID   int              `json:"workflow_id"`
	Status       string           `json:"status"`
	WorkflowType int              `json:"workflow_type"`
	WorkflowName string           `json:"workflow_name"`
	Title        string           `json:"title"`
	Workflow     *WorkflowSummary `json:"workflow"`
}

type WorkflowSummary struct {
	ID           int    `json:"id"`
	Status       string `json:"status"`
	WorkflowType int    `json:"workflow_type"`
	WorkflowName string `json:"workflow_name"`
}

func (w Workflow) DisplayName() string {
	if w.Workflow != nil && strings.TrimSpace(w.Workflow.WorkflowName) != "" {
		return w.Workflow.WorkflowName
	}
	if strings.TrimSpace(w.WorkflowName) != "" {
		return w.WorkflowName
	}
	return w.Title
}

func (w Workflow) EffectiveStatus() string {
	if w.Workflow != nil && strings.TrimSpace(w.Workflow.Status) != "" {
		return w.Workflow.Status
	}
	return w.Status
}

func (w Workflow) EffectiveWorkflowType() int {
	if w.WorkflowType != 0 {
		return w.WorkflowType
	}
	if w.Workflow != nil && w.Workflow.WorkflowType != 0 {
		return w.Workflow.WorkflowType
	}
	return 0
}

func (w Workflow) EffectiveID() int {
	if w.WorkflowID != 0 {
		return w.WorkflowID
	}
	return w.ID
}

func (w Workflow) IsApprovedLike() bool {
	status := strings.ToLower(strings.TrimSpace(w.EffectiveStatus()))
	switch status {
	case "approved", "workflow_finish", "workflow_review_pass", "finish", "done", "passed":
		return true
	default:
		return false
	}
}
