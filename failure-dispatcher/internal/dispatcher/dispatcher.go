package dispatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// Config holds configuration parameters for the failure dispatcher.
type Config struct {
	ProjectID   string
	GitHubOwner string
	GitHubRepo  string
	SecretID    string
	EventType   string
	GitHubToken string // Optional override for local testing
}

// SecretAccessor abstracts fetching secrets.
type SecretAccessor interface {
	GetSecret(ctx context.Context, projectID, secretID string) (string, error)
}

// GCPSecretAccessor retrieves secrets from Google Cloud Secret Manager.
type GCPSecretAccessor struct {
	client *secretmanager.Client
}

// NewGCPSecretAccessor creates a new GCPSecretAccessor.
func NewGCPSecretAccessor(ctx context.Context) (*GCPSecretAccessor, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}
	return &GCPSecretAccessor{client: client}, nil
}

// Close closes the underlying client.
func (a *GCPSecretAccessor) Close() error {
	if a.client != nil {
		return a.client.Close()
	}
	return nil
}

// GetSecret fetches the latest version of the specified secret.
func (a *GCPSecretAccessor) GetSecret(ctx context.Context, projectID, secretID string) (string, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", projectID, secretID)
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}
	result, err := a.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", name, err)
	}
	return string(result.Payload.Data), nil
}

// CloudMonitoringAlert represents the JSON payload Cloud Monitoring sends to Pub/Sub on alerts.
type CloudMonitoringAlert struct {
	Incident struct {
		IncidentID    string `json:"incident_id"`
		ResourceID    string `json:"resource_id"`
		ResourceName  string `json:"resource_name"`
		ResourceType  string `json:"resource_type"`
		ConditionName string `json:"condition_name"`
		Summary       string `json:"summary"`
		URL           string `json:"url"`
		StartedAt     int64  `json:"started_at"`
		EndedAt       int64  `json:"ended_at"`
		State         string `json:"state"`
	} `json:"incident"`
	Version string `json:"version"`
}

// DispatchPayload is the client_payload sent to GitHub repository_dispatch.
type DispatchPayload struct {
	JobName       string `json:"job_name"`
	ExecutionName string `json:"execution_name,omitempty"`
	IncidentID    string `json:"incident_id,omitempty"`
	Summary       string `json:"summary,omitempty"`
	ConditionName string `json:"condition_name,omitempty"`
	ResourceName  string `json:"resource_name,omitempty"`
	IncidentURL   string `json:"incident_url,omitempty"`
	TriggeredAt   string `json:"triggered_at"`
	RawDetails    string `json:"raw_details,omitempty"`
}

// RepositoryDispatchBody is the root body required by GitHub API.
type RepositoryDispatchBody struct {
	EventType     string          `json:"event_type"`
	ClientPayload DispatchPayload `json:"client_payload"`
}

// Dispatcher handles processing alert payloads and triggering GitHub Actions.
type Dispatcher struct {
	cfg        *Config
	secrets    SecretAccessor
	httpClient *http.Client
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(cfg *Config, secrets SecretAccessor, httpClient *http.Client) *Dispatcher {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.EventType == "" {
		cfg.EventType = "cloud-run-job-failure"
	}
	if cfg.GitHubOwner == "" {
		cfg.GitHubOwner = "5pirit5eal"
	}
	if cfg.GitHubRepo == "" {
		cfg.GitHubRepo = "wellpass-autoform"
	}
	if cfg.SecretID == "" {
		cfg.SecretID = "github-dispatch-token"
	}
	return &Dispatcher{
		cfg:        cfg,
		secrets:    secrets,
		httpClient: httpClient,
	}
}

// ParsePubSubData parses the incoming Pub/Sub message data into a DispatchPayload.
func ParsePubSubData(data []byte) DispatchPayload {
	payload := DispatchPayload{
		JobName:     "form-submission-job",
		TriggeredAt: time.Now().UTC().Format(time.RFC3339),
	}

	if len(data) == 0 {
		payload.Summary = "Cloud Run job failure detected (empty message)"
		return payload
	}

	var alert CloudMonitoringAlert
	if err := json.Unmarshal(data, &alert); err == nil && alert.Incident.IncidentID != "" {
		payload.IncidentID = alert.Incident.IncidentID
		payload.Summary = alert.Incident.Summary
		payload.ConditionName = alert.Incident.ConditionName
		payload.ResourceName = alert.Incident.ResourceName
		payload.IncidentURL = alert.Incident.URL
		if alert.Incident.ResourceName != "" {
			parts := strings.Split(alert.Incident.ResourceName, "/")
			payload.JobName = parts[len(parts)-1]
		}
		return payload
	}

	// Fallback to checking if it's already a custom JSON payload
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err == nil {
		if jn, ok := rawMap["job_name"].(string); ok && jn != "" {
			payload.JobName = jn
		}
		if s, ok := rawMap["summary"].(string); ok && s != "" {
			payload.Summary = s
		}
		if en, ok := rawMap["execution_name"].(string); ok && en != "" {
			payload.ExecutionName = en
		}
		payload.RawDetails = string(data)
		return payload
	}

	payload.Summary = string(data)
	return payload
}

// Dispatch sends a repository_dispatch event to GitHub.
func (d *Dispatcher) Dispatch(ctx context.Context, payload DispatchPayload) error {
	token := d.cfg.GitHubToken
	if token == "" && d.secrets != nil && d.cfg.ProjectID != "" {
		sec, err := d.secrets.GetSecret(ctx, d.cfg.ProjectID, d.cfg.SecretID)
		if err != nil {
			return fmt.Errorf("failed to fetch GitHub token from Secret Manager: %w", err)
		}
		token = strings.TrimSpace(sec)
	}

	if token == "" {
		return fmt.Errorf("GitHub token is empty; cannot trigger repository_dispatch")
	}

	dispatchBody := RepositoryDispatchBody{
		EventType:     d.cfg.EventType,
		ClientPayload: payload,
	}

	bodyBytes, err := json.Marshal(dispatchBody)
	if err != nil {
		return fmt.Errorf("failed to marshal dispatch body: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/dispatches", d.cfg.GitHubOwner, d.cfg.GitHubRepo)
	log.Printf("Triggering GitHub repository_dispatch: POST %s (event_type=%s, job=%s)...", url, d.cfg.EventType, payload.JobName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "wellpass-failure-dispatcher")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request to GitHub failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub repository_dispatch returned status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("Successfully triggered GitHub repository_dispatch for %s/%s (HTTP %d)", d.cfg.GitHubOwner, d.cfg.GitHubRepo, resp.StatusCode)
	return nil
}
