# ---------------------------------------------------------------------------------------------------------------------
# GitHub Actions Repository Variables (configured via integrations/github provider)
# ---------------------------------------------------------------------------------------------------------------------
locals {
  github_vars = {
    GCP_PROJECT_ID                  = var.project_id
    GCP_REGION                      = var.region
    GEMINI_LOCATION                 = var.gemini_location
    GCP_WORKLOAD_IDENTITY_PROVIDER  = "${google_iam_workload_identity_pool.github_pool.name}/providers/${google_iam_workload_identity_pool_provider.github_provider.workload_identity_pool_provider_id}"
    GCP_SERVICE_ACCOUNT             = google_service_account.github_actions.email
    RECEIPTS_SOURCE_BUCKET          = google_storage_bucket.unprocessed_receipts.name
    RECEIPTS_TARGET_BUCKET          = google_storage_bucket.processed_receipts.name
    RECEIPTS_FAILED_BUCKET          = google_storage_bucket.failed_receipts.name
    ARTIFACT_REGISTRY_REPOSITORY    = google_artifact_registry_repository.receipts.name
    BIGQUERY_DATASET                = google_bigquery_dataset.receipts.dataset_id
    BIGQUERY_TABLE                  = google_bigquery_table.processing_results.table_id
    GDRIVE_UPLOADER_SERVICE_ACCOUNT = google_service_account.gdrive_uploader.email
  }
}

resource "github_actions_variable" "variables" {
  for_each      = var.enable_github_resources ? local.github_vars : {}
  repository    = var.github_repository
  variable_name = each.key
  value         = each.value
}
