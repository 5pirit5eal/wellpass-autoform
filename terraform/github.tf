# ---------------------------------------------------------------------------------------------------------------------
# GitHub Actions Repository Variables & Secrets (configured via integrations/github provider)
# ---------------------------------------------------------------------------------------------------------------------
locals {
  github_vars = {
    GCP_PROJECT_ID                  = var.project_id
    GCP_REGION                      = var.region
    GEMINI_LOCATION                 = var.gemini_location
    GEMINI_MODEL                    = "gemini-3.5-flash-lite"
    GOOGLE_GENAI_USE_VERTEXAI       = "true"
    GCP_WORKLOAD_IDENTITY_PROVIDER  = "${google_iam_workload_identity_pool.github_pool.name}/providers/${google_iam_workload_identity_pool_provider.github_provider.workload_identity_pool_provider_id}"
    GCP_SERVICE_ACCOUNT             = google_service_account.github_actions.email
    RECEIPTS_SOURCE_BUCKET          = google_storage_bucket.unprocessed_receipts.name
    RECEIPTS_TARGET_BUCKET          = google_storage_bucket.processed_receipts.name
    RECEIPTS_SUBMITTED_BUCKET       = google_storage_bucket.submitted_receipts.name
    RECEIPTS_FAILED_BUCKET          = google_storage_bucket.failed_receipts.name
    ARTIFACT_REGISTRY_REPOSITORY    = google_artifact_registry_repository.receipts.name
    BIGQUERY_DATASET                = google_bigquery_dataset.receipts.dataset_id
    BIGQUERY_TABLE                  = google_bigquery_table.processing_results.table_id
    GDRIVE_UPLOADER_SERVICE_ACCOUNT = google_service_account.gdrive_uploader.email
    SUBMISSION_JOB_SERVICE_ACCOUNT  = google_service_account.form_submission_job.email
    WELLPASS_TYPEFORM_URL           = "https://egym.typeform.com/to/z5XBrNXf"
    WELLPASS_EMAIL                  = var.wellpass_email
    WELLPASS_FIRST_NAME             = var.wellpass_first_name
    WELLPASS_LAST_NAME              = var.wellpass_last_name
    WELLPASS_BIC                    = var.wellpass_bic
    SUBMISSION_DRY_RUN              = tostring(var.submission_dry_run)
  }
}

resource "github_actions_variable" "variables" {
  for_each      = var.enable_github_resources ? local.github_vars : {}
  repository    = var.github_repository
  variable_name = each.key
  value         = each.value
}

# Sensitive banking details stored as encrypted GitHub repository secrets
resource "github_actions_secret" "iban" {
  count       = var.enable_github_resources && var.wellpass_iban != "" ? 1 : 0
  repository  = var.github_repository
  secret_name = "WELLPASS_IBAN"
  value       = var.wellpass_iban
}
