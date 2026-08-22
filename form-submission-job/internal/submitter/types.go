package submitter

import (
	"context"
)

// SubmissionTicket holds the details for a single ticket within a submission.
type SubmissionTicket struct {
	ObjectName    string `json:"object_name"`
	PoolLabel     string `json:"pool_label"`
	Day           string `json:"day"`
	Month         string `json:"month"`
	Year          string `json:"year"`
	Price         string `json:"price"` // e.g. "5,44"
	FilePath      string `json:"file_path"`
	ReceiptNumber string `json:"receipt_number,omitempty"`
}

// SubmissionBatch holds all parameters required for one Typeform submission run.
type SubmissionBatch struct {
	BatchID        string             `json:"batch_id"`
	TypeformURL    string             `json:"typeform_url"`
	Email          string             `json:"email"`
	FullName       string             `json:"full_name"`
	IBAN           string             `json:"iban"`
	BIC            string             `json:"bic"`
	Tickets        []SubmissionTicket `json:"tickets"`
	DryRun         bool               `json:"dry_run"`
	Headless       bool               `json:"headless"`
	ScreenshotsDir string             `json:"screenshots_dir"`
}

// SubmissionResult contains the output and audit trail of a submission attempt.
type SubmissionResult struct {
	BatchID          string   `json:"batch_id"`
	Success          bool     `json:"success"`
	TicketsSubmitted int      `json:"tickets_submitted"`
	Screenshots      []string `json:"screenshots"`
	ErrorMessage     string   `json:"error_message,omitempty"`
}

// FormSubmitter defines the interface for submitting forms to Typeform.
type FormSubmitter interface {
	Submit(ctx context.Context, batch SubmissionBatch) (*SubmissionResult, error)
}
