output "source_bucket_name" {
  description = "Name of the source bucket for unprocessed receipts"
  value       = google_storage_bucket.unprocessed_receipts.name
}

output "processed_bucket_name" {
  description = "Name of the target archive bucket for processed receipts"
  value       = google_storage_bucket.processed_receipts.name
}

output "failed_bucket_name" {
  description = "Name of the bucket for failed receipts or conflicts"
  value       = google_storage_bucket.failed_receipts.name
}

output "bigquery_dataset_id" {
  description = "BigQuery dataset ID"
  value       = google_bigquery_dataset.receipts.dataset_id
}

output "bigquery_table_id" {
  description = "BigQuery processing results table ID"
  value       = google_bigquery_table.processing_results.table_id
}

output "artifact_registry_repository_name" {
  description = "Name of the Artifact Registry repository"
  value       = google_artifact_registry_repository.receipts.name
}

output "function_service_account_email" {
  description = "Email of the Cloud Run function runtime service account"
  value       = google_service_account.receipts_function.email
}

output "github_actions_service_account_email" {
  description = "Email of the GitHub Actions CI/CD service account"
  value       = google_service_account.github_actions.email
}

output "gdrive_uploader_service_account_email" {
  description = "Email of the service account used to upload receipts from Google Drive to GCS"
  value       = google_service_account.gdrive_uploader.email
}

output "workload_identity_provider" {
  description = "Full identifier of the Workload Identity Provider for GitHub Actions"
  value       = "${google_iam_workload_identity_pool.github_pool.name}/providers/${google_iam_workload_identity_pool_provider.github_provider.workload_identity_pool_provider_id}"
}
