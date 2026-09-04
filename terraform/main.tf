locals {
  source_bucket    = var.source_bucket_name != "" ? var.source_bucket_name : "${var.project_id}-receipts-unprocessed"
  processed_bucket = var.processed_bucket_name != "" ? var.processed_bucket_name : "${var.project_id}-receipts-processed"
  submitted_bucket = var.submitted_bucket_name != "" ? var.submitted_bucket_name : "${var.project_id}-receipts-submitted"
  failed_bucket    = var.failed_bucket_name != "" ? var.failed_bucket_name : "${var.project_id}-receipts-failed"
  bq_location      = var.bigquery_location != "" ? var.bigquery_location : var.region

  services = [
    "run.googleapis.com",
    "cloudfunctions.googleapis.com",
    "cloudscheduler.googleapis.com",
    "eventarc.googleapis.com",
    "pubsub.googleapis.com",
    "storage.googleapis.com",
    "aiplatform.googleapis.com",
    "bigquery.googleapis.com",
    "artifactregistry.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "cloudbuild.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "secretmanager.googleapis.com"
  ]
}

resource "google_project_service" "apis" {
  for_each                   = var.enable_apis ? toset(local.services) : toset([])
  project                    = var.project_id
  service                    = each.key
  disable_on_destroy         = false
  disable_dependent_services = false
}
