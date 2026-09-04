variable "project_id" {
  description = "The Google Cloud Project ID"
  type        = string
}

variable "region" {
  description = "The Google Cloud region for resources"
  type        = string
  default     = "europe-west3"
}

variable "gemini_location" {
  description = "The location for Gemini API or Vertex AI (defaults to var.region)"
  type        = string
  default     = "eu"
}

variable "source_bucket_name" {
  description = "Optional explicit name for the unprocessed receipts source bucket (auto-generated if empty)"
  type        = string
  default     = ""
}

variable "processed_bucket_name" {
  description = "Optional explicit name for the processed receipts transitional standard bucket (auto-generated if empty)"
  type        = string
  default     = ""
}

variable "submitted_bucket_name" {
  description = "Optional explicit name for the submitted receipts archive bucket (auto-generated if empty)"
  type        = string
  default     = ""
}

variable "failed_bucket_name" {
  description = "Optional explicit name for the failed processing coldline bucket (auto-generated if empty)"
  type        = string
  default     = ""
}

variable "bigquery_dataset_id" {
  description = "BigQuery dataset ID for storing receipts processing results"
  type        = string
  default     = "receipts_processing"
}

variable "bigquery_location" {
  description = "BigQuery dataset location (defaults to var.region or EU)"
  type        = string
  default     = ""
}

variable "artifact_registry_repo_id" {
  description = "Artifact Registry repository ID for container images"
  type        = string
  default     = "golang"
}

variable "github_owner" {
  description = "GitHub repository owner (user or organization)"
  type        = string
  default     = "5pirit5eal"
}

variable "github_repository" {
  description = "GitHub repository name"
  type        = string
  default     = "wellpass-autoform"
}

variable "enable_github_resources" {
  description = "Whether to manage GitHub Actions environment variables via Terraform"
  type        = bool
  default     = true
}

variable "enable_apis" {
  description = "Whether to enable necessary Google Cloud service APIs via Terraform"
  type        = bool
  default     = true
}

# ---------------------------------------------------------------------------------------------------------------------
# Wellpass Form Submission Configuration
# ---------------------------------------------------------------------------------------------------------------------
variable "wellpass_email" {
  description = "Member email registered with EGYM Wellpass"
  type        = string
  default     = ""
}

variable "wellpass_first_name" {
  description = "Member first name"
  type        = string
  default     = ""
}

variable "wellpass_last_name" {
  description = "Member last name"
  type        = string
  default     = ""
}

variable "wellpass_iban" {
  description = "Member bank account IBAN (without spaces)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "wellpass_bic" {
  description = "Member bank account BIC"
  type        = string
  default     = ""
}

variable "submission_dry_run" {
  description = "Whether the Cloud Run Job runs in dry-run mode (prevents final form submit)"
  type        = bool
  default     = true
}

variable "scheduler_cron" {
  description = "Cron schedule for monthly submission trigger (defaults to 2nd of each month at 08:00)"
  type        = string
  default     = "0 8 2 * *"
}

variable "scheduler_time_zone" {
  description = "Time zone for Cloud Scheduler job"
  type        = string
  default     = "Europe/Berlin"
}

variable "github_dispatch_token" {
  description = "GitHub Personal Access Token (PAT with actions:write or repo scope) used by GCP to trigger repository_dispatch workflows"
  type        = string
  default     = ""
  sensitive   = true
}
