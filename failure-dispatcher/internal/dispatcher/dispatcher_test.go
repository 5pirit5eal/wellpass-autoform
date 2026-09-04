package dispatcher

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockSecretAccessor struct {
	secret string
	err    error
}

func (m *mockSecretAccessor) GetSecret(ctx context.Context, projectID, secretID string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.secret, nil
}

func TestParsePubSubData_CloudMonitoringAlert(t *testing.T) {
	alertJSON := `{
		"incident": {
			"incident_id": "0.abcdef123",
			"resource_id": "form-submission-job",
			"resource_name": "receipt-processing-egym Cloud Run Job labels {project_id=receipt-processing-egym, job_name=form-submission-job}",
			"resource_type": "cloud_run_job",
			"resource": {
				"type": "cloud_run_job",
				"labels": {
					"job_name": "form-submission-job",
					"project_id": "receipt-processing-egym"
				}
			},
			"condition_name": "Job Failed Executions",
			"summary": "Cloud Run Job form-submission-job has failed executions",
			"url": "https://console.cloud.google.com/monitoring/alerting/incidents/0.abcdef123",
			"state": "open"
		},
		"version": "1.2"
	}`

	payload := ParsePubSubData([]byte(alertJSON))

	if payload.JobName != "form-submission-job" {
		t.Errorf("expected job_name form-submission-job, got %q", payload.JobName)
	}
	if payload.IncidentID != "0.abcdef123" {
		t.Errorf("expected incident_id 0.abcdef123, got %q", payload.IncidentID)
	}
	if payload.ConditionName != "Job Failed Executions" {
		t.Errorf("expected condition_name 'Job Failed Executions', got %q", payload.ConditionName)
	}
	if payload.IncidentURL != "https://console.cloud.google.com/monitoring/alerting/incidents/0.abcdef123" {
		t.Errorf("unexpected incident URL: %q", payload.IncidentURL)
	}
}

func TestParsePubSubData_CloudMonitoringAlert_ResourceNameOnly(t *testing.T) {
	alertJSON := `{
		"incident": {
			"incident_id": "0.abcdef123",
			"resource_name": "receipt-processing-egym Cloud Run Job labels {project_id=receipt-processing-egym, job_name=form-submission-job}",
			"condition_name": "Job Failed Executions"
		}
	}`

	payload := ParsePubSubData([]byte(alertJSON))

	if payload.JobName != "form-submission-job" {
		t.Errorf("expected job_name form-submission-job, got %q", payload.JobName)
	}
}

func TestParsePubSubData_CustomJSON(t *testing.T) {
	customJSON := `{"job_name": "custom-job", "execution_name": "exec-99", "summary": "Playwright timeout"}`

	payload := ParsePubSubData([]byte(customJSON))

	if payload.JobName != "custom-job" {
		t.Errorf("expected custom-job, got %q", payload.JobName)
	}
	if payload.ExecutionName != "exec-99" {
		t.Errorf("expected exec-99, got %q", payload.ExecutionName)
	}
	if payload.Summary != "Playwright timeout" {
		t.Errorf("expected summary 'Playwright timeout', got %q", payload.Summary)
	}
}

func TestDispatcher_Dispatch_Success(t *testing.T) {
	var capturedAuth string
	var capturedBody RepositoryDispatchBody

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("missing or incorrect Accept header: %s", r.Header.Get("Accept"))
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &capturedBody)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &Config{
		ProjectID:   "my-proj",
		GitHubOwner: "5pirit5eal",
		GitHubRepo:  "wellpass-autoform",
		EventType:   "cloud-run-job-failure",
	}

	secrets := &mockSecretAccessor{secret: "ghp_test_token_12345"}
	d := NewDispatcher(cfg, secrets, server.Client())

	// Override URL routing by custom RoundTripper
	serverURL := server.URL
	d.httpClient.Transport = &rewriteTransport{targetURL: serverURL, original: http.DefaultTransport}

	payload := DispatchPayload{
		JobName:    "form-submission-job",
		IncidentID: "inc-123",
		Summary:    "Task failed",
	}

	err := d.Dispatch(context.Background(), payload)
	if err != nil {
		t.Fatalf("expected dispatch to succeed, got %v", err)
	}

	if capturedAuth != "Bearer ghp_test_token_12345" {
		t.Errorf("expected auth 'Bearer ghp_test_token_12345', got %q", capturedAuth)
	}
	if capturedBody.EventType != "cloud-run-job-failure" {
		t.Errorf("expected event_type 'cloud-run-job-failure', got %q", capturedBody.EventType)
	}
	if capturedBody.ClientPayload.IncidentID != "inc-123" {
		t.Errorf("expected client_payload incident_id 'inc-123', got %q", capturedBody.ClientPayload.IncidentID)
	}
}

func TestDispatcher_Dispatch_RepoWithSlash(t *testing.T) {
	var capturedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := &Config{
		ProjectID:   "my-proj",
		GitHubOwner: "5pirit5eal",
		GitHubRepo:  "5pirit5eal/wellpass-autoform", // Contains slash
		EventType:   "cloud-run-job-failure",
	}

	secrets := &mockSecretAccessor{secret: "ghp_test"}
	d := NewDispatcher(cfg, secrets, server.Client())
	d.httpClient.Transport = &rewriteTransport{targetURL: server.URL, original: http.DefaultTransport}

	err := d.Dispatch(context.Background(), DispatchPayload{JobName: "form-submission-job"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := "/repos/5pirit5eal/wellpass-autoform/dispatches"
	if capturedPath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, capturedPath)
	}
}

type rewriteTransport struct {
	targetURL string
	original  http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite host and scheme to point to httptest server
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.targetURL, "http://")
	return t.original.RoundTrip(req)
}
