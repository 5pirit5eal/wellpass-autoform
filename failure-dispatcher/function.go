package dispatcherfunc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/wellpass-autoform/failure-dispatcher/internal/dispatcher"
)

var (
	initOnce   sync.Once
	dispatchOp *dispatcher.Dispatcher
	initErr    error
)

// MessagePublishedData represents the payload of a Pub/Sub CloudEvent.
type MessagePublishedData struct {
	Message struct {
		Data        []byte            `json:"data"`
		Attributes  map[string]string `json:"attributes"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

func init() {
	functions.CloudEvent("DispatchFailure", DispatchFailure)
}

func initializeDependencies(ctx context.Context) error {
	initOnce.Do(func() {
		projectID := os.Getenv("PROJECT_ID")
		if projectID == "" {
			projectID = os.Getenv("GCP_PROJECT_ID")
		}

		owner := os.Getenv("GITHUB_OWNER")
		if owner == "" {
			owner = "5pirit5eal"
		}

		repo := os.Getenv("GITHUB_REPOSITORY")
		if repo == "" {
			repo = "wellpass-autoform"
		}

		secretID := os.Getenv("ACTION_DISPATCH_SECRET_ID")
		if secretID == "" {
			secretID = "github-dispatch-token"
		}

		eventType := os.Getenv("GITHUB_EVENT_TYPE")
		if eventType == "" {
			eventType = "cloud-run-job-failure"
		}

		cfg := &dispatcher.Config{
			ProjectID:   projectID,
			GitHubOwner: owner,
			GitHubRepo:  repo,
			SecretID:    secretID,
			EventType:   eventType,
			GitHubToken: os.Getenv("GITHUB_TOKEN"), // Optional local override
		}

		var secretAccessor dispatcher.SecretAccessor
		if cfg.GitHubToken == "" && projectID != "" {
			acc, err := dispatcher.NewGCPSecretAccessor(ctx)
			if err != nil {
				initErr = fmt.Errorf("failed to create GCP secret accessor: %w", err)
				return
			}
			secretAccessor = acc
		}

		dispatchOp = dispatcher.NewDispatcher(cfg, secretAccessor, nil)
	})
	return initErr
}

// DispatchFailure handles Pub/Sub CloudEvents when a Cloud Run Job failure alert fires.
func DispatchFailure(ctx context.Context, e cloudevents.Event) error {
	log.Printf("Received CloudEvent ID: %s, Type: %s, Source: %s", e.ID(), e.Type(), e.Source())

	if err := initializeDependencies(ctx); err != nil {
		log.Printf("Initialization error: %v", err)
		return err
	}

	var pubsubData MessagePublishedData
	if err := json.Unmarshal(e.Data(), &pubsubData); err != nil {
		log.Printf("Error unmarshaling CloudEvent data: %v", err)
		return fmt.Errorf("failed to parse pubsub cloud event data: %w", err)
	}

	payload := dispatcher.ParsePubSubData(pubsubData.Message.Data)
	log.Printf("Parsed failure event for job: %s (Incident: %s)", payload.JobName, payload.IncidentID)

	if err := dispatchOp.Dispatch(ctx, payload); err != nil {
		log.Printf("Error dispatching failure to GitHub: %v", err)
		return err
	}

	log.Printf("Successfully processed and forwarded failure alert to GitHub Actions.")
	return nil
}
