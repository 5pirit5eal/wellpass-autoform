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

variable "environment" {
  description = "Environment identifier (e.g. dev, prod)"
  type        = string
  default     = "prod"
}

variable "source_bucket_name" {
  description = "Optional explicit name for the unprocessed receipts source bucket (auto-generated if empty)"
  type        = string
  default     = ""
}

variable "processed_bucket_name" {
  description = "Optional explicit name for the processed receipts target archive bucket (auto-generated if empty)"
  type        = string
  default     = ""
}

variable "failed_bucket_name" {
  description = "Optional explicit name for the failed processing bucket (auto-generated if empty)"
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
