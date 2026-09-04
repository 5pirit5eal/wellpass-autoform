package submitter

import (
	"context"
	"fmt"
)

// MockSubmitter is a mock implementation of FormSubmitter for testing.
type MockSubmitter struct {
	SubmittedBatches []SubmissionBatch
	ShouldFail       bool
}

// Submit records the batch and returns a successful or failed result.
func (m *MockSubmitter) Submit(ctx context.Context, batch SubmissionBatch) (*SubmissionResult, error) {
	if m.ShouldFail {
		return &SubmissionResult{
			BatchID:     batch.BatchID,
			Success:     false,
			Screenshots: []string{"/tmp/mock_screenshot.png"},
		}, fmt.Errorf("mock submit error")
	}
	m.SubmittedBatches = append(m.SubmittedBatches, batch)
	return &SubmissionResult{
		BatchID:          batch.BatchID,
		Success:          true,
		TicketsSubmitted: len(batch.Tickets),
		Screenshots:      []string{"/tmp/mock_screenshot.png"},
	}, nil
}
