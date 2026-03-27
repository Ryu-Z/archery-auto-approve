package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"archery-auto-approve/config"
	"archery-auto-approve/model"
	"archery-auto-approve/utils"
)

func TestListAndApproveWorkflow(t *testing.T) {
	var (
		tokenCalls    int
		approveCalls  int
		detailQueries int
	)

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			tokenCalls++
			return jsonResponse(t, http.StatusOK, map[string]string{
				"access":  "test-access-token",
				"refresh": "test-refresh-token",
			}), nil
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workflow/":
			if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
				t.Fatalf("unexpected authorization header: %s", got)
			}
			if got := r.URL.Query().Get("workflow_id"); got == "101" {
				detailQueries++
				return jsonResponse(t, http.StatusOK, map[string]any{
					"count": 1,
					"results": []model.Workflow{
						{ID: 101, WorkflowID: 101, WorkflowType: 1, Workflow: &model.WorkflowSummary{Status: "workflow_manreviewing", WorkflowName: "test-workflow"}},
					},
				}), nil
			}
			if got := r.URL.Query().Get("workflow__status"); got != "workflow_manreviewing" {
				t.Fatalf("unexpected status query: %s", got)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"count": 1,
				"results": []model.Workflow{
					{ID: 101, WorkflowID: 101, WorkflowType: 1, Workflow: &model.WorkflowSummary{Status: "workflow_manreviewing", WorkflowName: "test-workflow"}},
				},
			}), nil
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workflow/audit/":
			approveCalls++
			if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
				t.Fatalf("unexpected authorization header: %s", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if !strings.Contains(string(body), `"audit_type":"pass"`) {
				t.Fatalf("unexpected approve body: %s", string(body))
			}
			if !strings.Contains(string(body), `"workflow_type":1`) {
				t.Fatalf("unexpected workflow_type in approve body: %s", string(body))
			}
			return jsonResponse(t, http.StatusOK, map[string]string{"status": "ok"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})

	cfg := &config.Config{
		Archery: config.ArcheryConfig{
			BaseURL:             "http://archery.test",
			Username:            "admin",
			Password:            "secret",
			TokenTTL:            3600,
			AuthScheme:          "Bearer",
			WorkflowListPath:    "/api/v1/workflow/",
			WorkflowApprovePath: "/api/v1/workflow/audit/",
			WorkflowApproveAlt:  "/api/workflow/approve/",
			TokenPath:           "/api/token/",
			TokenRefreshPath:    "/api/auth/token/refresh/",
			LoginPath:           "/api/v1/user/login/",
		},
		RetryCount:      2,
		RetryBackoffSec: 0,
	}

	logger, err := utils.NewLogger("error")
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	client := NewClient(cfg, logger)
	client.httpClient.Transport = transport
	workflows, err := client.ListPendingWorkflows(context.Background(), []string{"workflow_manreviewing"})
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("unexpected workflow count: %d", len(workflows))
	}

	if err := client.ApproveWorkflow(context.Background(), workflows[0], "系统自动审批（非工作时间）", "auto-bot"); err != nil {
		t.Fatalf("approve workflow: %v", err)
	}

	if tokenCalls != 1 {
		t.Fatalf("unexpected token call count: %d", tokenCalls)
	}
	if approveCalls != 1 {
		t.Fatalf("unexpected approve call count: %d", approveCalls)
	}
	if detailQueries != 1 {
		t.Fatalf("unexpected detail query count: %d", detailQueries)
	}
}

func TestApproveWorkflowSkipsApprovedWorkflow(t *testing.T) {
	var (
		tokenCalls    int
		approveCalls  int
		detailQueries int
	)

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			tokenCalls++
			return jsonResponse(t, http.StatusOK, map[string]string{
				"access":  "test-access-token",
				"refresh": "test-refresh-token",
			}), nil
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workflow/":
			detailQueries++
			return jsonResponse(t, http.StatusOK, map[string]any{
				"count": 1,
				"results": []model.Workflow{
					{ID: 101, WorkflowID: 101, WorkflowType: 2, Workflow: &model.WorkflowSummary{Status: "workflow_review_pass", WorkflowName: "already-approved"}},
				},
			}), nil
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/workflow/audit/":
			approveCalls++
			return jsonResponse(t, http.StatusOK, map[string]string{"status": "ok"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			return nil, nil
		}
	})

	cfg := &config.Config{
		Archery: config.ArcheryConfig{
			BaseURL:             "http://archery.test",
			Username:            "admin",
			Password:            "secret",
			TokenTTL:            3600,
			AuthScheme:          "Bearer",
			WorkflowListPath:    "/api/v1/workflow/",
			WorkflowApprovePath: "/api/v1/workflow/audit/",
			WorkflowApproveAlt:  "/api/workflow/approve/",
			TokenPath:           "/api/token/",
			TokenRefreshPath:    "/api/auth/token/refresh/",
			LoginPath:           "/api/v1/user/login/",
		},
		RetryCount:      2,
		RetryBackoffSec: 0,
	}

	logger, err := utils.NewLogger("error")
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	client := NewClient(cfg, logger)
	client.httpClient.Transport = transport
	if err := client.ApproveWorkflow(context.Background(), model.Workflow{ID: 101, WorkflowID: 101, WorkflowType: 2}, "系统自动审批（非工作时间）", "admin"); err != nil {
		t.Fatalf("approve workflow: %v", err)
	}

	if tokenCalls != 1 {
		t.Fatalf("unexpected token call count: %d", tokenCalls)
	}
	if detailQueries != 1 {
		t.Fatalf("unexpected detail query count: %d", detailQueries)
	}
	if approveCalls != 0 {
		t.Fatalf("unexpected approve call count: %d", approveCalls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(t *testing.T, statusCode int, data any) *http.Response {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(raw))),
	}
}
