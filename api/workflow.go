package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"archery-auto-approve/model"
)

type approvalRequest struct {
	WorkflowID   int    `json:"workflow_id"`
	AuditType    string `json:"audit_type"`
	AuditRemark  string `json:"audit_remark"`
	Engineer     string `json:"engineer,omitempty"`
	WorkflowType int    `json:"workflow_type,omitempty"`
}

func (c *Client) ListPendingWorkflows(ctx context.Context, statuses []string) ([]model.Workflow, error) {
	var (
		mu        sync.Mutex
		seen      = make(map[int]struct{})
		result    []model.Workflow
		lastError error
	)

	for _, status := range statuses {
		workflows, err := c.listWorkflowsByStatus(ctx, c.cfg.Archery.WorkflowListPath, status)
		if err != nil {
			lastError = err
			c.logger.Error("list workflow failed", fieldStatus(status), fieldError(err))
			continue
		}

		mu.Lock()
		for _, wf := range workflows {
			if _, ok := seen[wf.ID]; ok {
				continue
			}
			seen[wf.ID] = struct{}{}
			result = append(result, wf)
		}
		mu.Unlock()
	}

	if len(result) == 0 && lastError != nil {
		return nil, lastError
	}
	return result, nil
}

func (c *Client) GetWorkflow(ctx context.Context, workflowID int) (*model.Workflow, error) {
	path, err := withWorkflowIDQuery(c.cfg.Archery.WorkflowListPath, workflowID)
	if err != nil {
		return nil, err
	}

	var respBody []byte
	err = c.withRetry(ctx, workflowID, func(runCtx context.Context) error {
		data, _, callErr := c.doJSON(runCtx, http.MethodGet, path, nil, true)
		if callErr != nil {
			return callErr
		}
		respBody = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	workflows, err := decodeWorkflowList(respBody)
	if err != nil {
		return nil, err
	}
	if len(workflows) == 0 {
		return nil, fmt.Errorf("workflow %d not found", workflowID)
	}
	return &workflows[0], nil
}

func (c *Client) listWorkflowsByStatus(ctx context.Context, basePath, status string) ([]model.Workflow, error) {
	path, err := withStatusQuery(basePath, status)
	if err != nil {
		return nil, err
	}

	var respBody []byte
	err = c.withRetry(ctx, 0, func(runCtx context.Context) error {
		data, _, callErr := c.doJSON(runCtx, http.MethodGet, path, nil, true)
		if callErr != nil {
			return callErr
		}
		respBody = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	workflows, err := decodeWorkflowList(respBody)
	if err != nil {
		return nil, err
	}

	return workflows, nil
}

func (c *Client) ApproveWorkflow(ctx context.Context, workflow model.Workflow, remark, approver string) error {
	latestWorkflow, err := c.GetWorkflow(ctx, workflow.EffectiveID())
	if err != nil {
		return fmt.Errorf("pre-check workflow status: %w", err)
	}
	if latestWorkflow.IsApprovedLike() {
		c.logger.Info("skip approve because workflow already approved",
			fieldWorkflowID(latestWorkflow.EffectiveID()),
			fieldWorkflowType(latestWorkflow.EffectiveWorkflowType()),
			fieldStatus(latestWorkflow.EffectiveStatus()),
		)
		return nil
	}

	workflowType := latestWorkflow.EffectiveWorkflowType()
	if workflowType == 0 {
		workflowType = 2
	}

	payload := approvalRequest{
		WorkflowID:   latestWorkflow.EffectiveID(),
		AuditType:    "pass",
		AuditRemark:  remark,
		Engineer:     approver,
		WorkflowType: workflowType,
	}

	primaryPath := c.cfg.Archery.WorkflowApprovePath
	err = c.withRetry(ctx, latestWorkflow.EffectiveID(), func(runCtx context.Context) error {
		_, _, callErr := c.doJSON(runCtx, http.MethodPost, primaryPath, payload, true)
		return callErr
	})
	if err == nil {
		return nil
	}

	c.logger.Warn("approve via primary endpoint failed, fallback to alternate endpoint", fieldWorkflowID(latestWorkflow.EffectiveID()), fieldWorkflowType(workflowType), fieldPath(primaryPath), fieldError(err))

	return c.withRetry(ctx, latestWorkflow.EffectiveID(), func(runCtx context.Context) error {
		_, _, callErr := c.doJSON(runCtx, http.MethodPost, c.cfg.Archery.WorkflowApproveAlt, payload, true)
		return callErr
	})
}

func (c *Client) withRetry(ctx context.Context, workflowID int, fn func(context.Context) error) error {
	var lastErr error

	for attempt := 1; attempt <= c.cfg.RetryCount; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		runCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		err := fn(runCtx)
		cancel()
		if err == nil {
			return nil
		}

		lastErr = err
		c.logger.Warn("request attempt failed",
			fieldWorkflowID(workflowID),
			fieldAttempt(attempt),
			fieldRetryMax(c.cfg.RetryCount),
			fieldError(err),
		)

		if attempt == c.cfg.RetryCount {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.cfg.RetryBackoff()):
		}
	}

	return lastErr
}

func withStatusQuery(basePath, status string) (string, error) {
	parsed, err := url.Parse(basePath)
	if err != nil {
		return "", fmt.Errorf("parse workflow path: %w", err)
	}
	query := parsed.Query()
	query.Set("workflow__status", status)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func withWorkflowIDQuery(basePath string, workflowID int) (string, error) {
	parsed, err := url.Parse(basePath)
	if err != nil {
		return "", fmt.Errorf("parse workflow path: %w", err)
	}
	query := parsed.Query()
	query.Set("workflow_id", fmt.Sprintf("%d", workflowID))
	query.Set("page", "1")
	query.Set("size", "1")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func decodeWorkflowList(data []byte) ([]model.Workflow, error) {
	var list []model.Workflow
	if err := json.Unmarshal(data, &list); err == nil {
		return list, nil
	}

	var wrapped struct {
		Results []model.Workflow `json:"results"`
		Data    []model.Workflow `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("decode workflow list: %w", err)
	}
	if len(wrapped.Results) > 0 {
		return wrapped.Results, nil
	}
	if len(wrapped.Data) > 0 {
		return wrapped.Data, nil
	}
	return []model.Workflow{}, nil
}
