# Wellpass Autoform

Automated swimming pool receipt processing and reimbursement submission for EGYM Wellpass.

## Project Structure

- **`receipts-function/`**: Cloud Run Function in Go 1.26 that processes incoming receipts (PDF/TXT) uploaded to Google Cloud Storage using Gemini 3.5 Flash Lite (`gemini-3.5-flash-lite`), extracts structured metadata, prevents conflicts, and archives processed receipts.
- **`terraform/`**: Infrastructure as Code (IaC) provisioning Cloud Storage buckets, IAM service accounts, BigQuery dataset/table, Artifact Registry, and Workload Identity Federation for GitHub Actions.
- **`form-submission-job/`**: Cloud Run job for automated form submission.
- **`examples/`**: Sample swimming pool receipts and tickets.
