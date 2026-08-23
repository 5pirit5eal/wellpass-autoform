# ---------------------------------------------------------------------------------------------------------------------
# 1. Cloud Run Function Runtime Service Account
# ---------------------------------------------------------------------------------------------------------------------
resource "google_service_account" "receipts_function" {
  account_id   = "receipts-function-sa"
  display_name = "Receipts Processing Cloud Run Function Service Account"
  description  = "Runtime service account used by the receipts processing Cloud Run function"

  depends_on = [google_project_service.apis]
}

# Project-level roles for Vertex AI, Logging, BigQuery, and Eventarc
resource "google_project_iam_member" "receipts_function_vertex" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.receipts_function.email}"
}

resource "google_project_iam_member" "receipts_function_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.receipts_function.email}"
}

resource "google_project_iam_member" "receipts_function_event_receiver" {
  project = var.project_id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.receipts_function.email}"
}

resource "google_project_iam_member" "receipts_function_bq_job_user" {
  project = var.project_id
  role    = "roles/bigquery.jobUser"
  member  = "serviceAccount:${google_service_account.receipts_function.email}"
}

# Bucket-level roles for the Function Service Account
resource "google_storage_bucket_iam_member" "unprocessed_reader" {
  bucket = google_storage_bucket.unprocessed_receipts.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.receipts_function.email}"
}

resource "google_storage_bucket_iam_member" "processed_admin" {
  bucket = google_storage_bucket.processed_receipts.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.receipts_function.email}"
}

resource "google_storage_bucket_iam_member" "failed_admin" {
  bucket = google_storage_bucket.failed_receipts.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.receipts_function.email}"
}

# BigQuery dataset editor role for writing results
resource "google_bigquery_dataset_iam_member" "receipts_bq_editor" {
  dataset_id = google_bigquery_dataset.receipts.dataset_id
  role       = "roles/bigquery.dataEditor"
  member     = "serviceAccount:${google_service_account.receipts_function.email}"
}

# ---------------------------------------------------------------------------------------------------------------------
# 2. Workload Identity Federation (WIF) for GitHub Actions CI/CD
# ---------------------------------------------------------------------------------------------------------------------
resource "google_iam_workload_identity_pool" "github_pool" {
  workload_identity_pool_id = "github-actions-pool"
  display_name              = "GitHub Actions Pool"
  description               = "Workload Identity Pool for GitHub Actions CI/CD workflows"

  depends_on = [google_project_service.apis]
}

resource "google_iam_workload_identity_pool_provider" "github_provider" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                       = "GitHub Provider"
  description                        = "OIDC identity provider for GitHub repository ${var.github_owner}/${var.github_repository}"

  attribute_mapping = {
    "google.subject"             = "assertion.sub"
    "attribute.actor"            = "assertion.actor"
    "attribute.repository"       = "assertion.repository"
    "attribute.repository_owner" = "assertion.repository_owner"
  }

  attribute_condition = "assertion.repository_owner == '${var.github_owner}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# ---------------------------------------------------------------------------------------------------------------------
# 3. GitHub Actions CI/CD Service Account
# ---------------------------------------------------------------------------------------------------------------------
resource "google_service_account" "github_actions" {
  account_id   = "github-actions-sa"
  display_name = "GitHub Actions CI/CD Deployment Service Account"
  description  = "Service account impersonated by GitHub Actions workflows via Workload Identity Federation"

  depends_on = [google_project_service.apis]
}

# Allow GitHub Actions repository to impersonate github-actions-sa via WIF
resource "google_service_account_iam_member" "github_actions_wif_binding" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${var.github_owner}/${var.github_repository}"
}

# Allow GitHub Actions SA to act as the Cloud Run function runtime service account
resource "google_service_account_iam_member" "github_actions_act_as_function_sa" {
  service_account_id = google_service_account.receipts_function.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.github_actions.email}"
}

# Project data source to determine default compute service account for Cloud Build / Gen2 deployment
data "google_project" "current" {
  project_id = var.project_id
}

# Allow GitHub Actions SA to act as default compute engine SA (used by Cloud Functions Gen2 / Cloud Build)
resource "google_project_iam_member" "github_actions_act_as_compute_sa" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# Grant CI/CD deployment permissions to github-actions-sa
resource "google_project_iam_member" "github_actions_cf_developer" {
  project = var.project_id
  role    = "roles/cloudfunctions.developer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_run_developer" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_ar_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_storage_admin" {
  project = var.project_id
  role    = "roles/storage.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_eventarc_admin" {
  project = var.project_id
  role    = "roles/eventarc.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_cloudbuild_editor" {
  project = var.project_id
  role    = "roles/cloudbuild.builds.editor"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_scheduler_admin" {
  project = var.project_id
  role    = "roles/cloudscheduler.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_project_iam_member" "github_actions_project_viewer" {
  project = var.project_id
  role    = "roles/viewer"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# ---------------------------------------------------------------------------------------------------------------------
# 4. Google Drive to GCS Uploader Service Account
# ---------------------------------------------------------------------------------------------------------------------
resource "google_service_account" "gdrive_uploader" {
  account_id   = "gdrive-uploader-sa"
  display_name = "Google Drive Receipts Uploader Service Account"
  description  = "Service account used to upload receipts from Google Drive to the unprocessed Cloud Storage source bucket"

  depends_on = [google_project_service.apis]
}

# Grant storage admin role on the unprocessed (source) receipts bucket
resource "google_storage_bucket_iam_member" "gdrive_uploader_creator" {
  bucket = google_storage_bucket.unprocessed_receipts.name
  role   = "roles/storage.admin"
  member = "serviceAccount:${google_service_account.gdrive_uploader.email}"
}

# ---------------------------------------------------------------------------------------------------------------------
# 5. Form Submission Cloud Run Job Runtime Service Account
# ---------------------------------------------------------------------------------------------------------------------
resource "google_service_account" "form_submission_job" {
  account_id   = "form-submission-job-sa"
  display_name = "Form Submission Job Service Account"
  description  = "Runtime service account used by the form submission Cloud Run job"

  depends_on = [google_project_service.apis]
}

resource "google_project_iam_member" "form_submission_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.form_submission_job.email}"
}

# Bucket access for the Job Service Account (read processed, write submitted archive, write failed coldline, delete from processed)
resource "google_storage_bucket_iam_member" "form_submission_processed_admin" {
  bucket = google_storage_bucket.processed_receipts.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.form_submission_job.email}"
}

resource "google_storage_bucket_iam_member" "form_submission_submitted_admin" {
  bucket = google_storage_bucket.submitted_receipts.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.form_submission_job.email}"
}

resource "google_storage_bucket_iam_member" "form_submission_failed_admin" {
  bucket = google_storage_bucket.failed_receipts.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.form_submission_job.email}"
}

# Allow GitHub Actions to act as the job runtime service account
resource "google_service_account_iam_member" "github_actions_act_as_job_sa" {
  service_account_id = google_service_account.form_submission_job.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.github_actions.email}"
}

# ---------------------------------------------------------------------------------------------------------------------
# 6. Cloud Scheduler Service Account to trigger Cloud Run Job
# ---------------------------------------------------------------------------------------------------------------------
resource "google_service_account" "scheduler_job_invoker" {
  account_id   = "scheduler-job-invoker-sa"
  display_name = "Cloud Scheduler Job Invoker Service Account"
  description  = "Service account used by Cloud Scheduler to invoke the form submission Cloud Run job"

  depends_on = [google_project_service.apis]
}

resource "google_project_iam_member" "scheduler_run_invoker" {
  project = var.project_id
  role    = "roles/run.invoker"
  member  = "serviceAccount:${google_service_account.scheduler_job_invoker.email}"
}

# Allow GitHub Actions to configure Cloud Scheduler with the scheduler invoker service account
resource "google_service_account_iam_member" "github_actions_act_as_scheduler_sa" {
  service_account_id = google_service_account.scheduler_job_invoker.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.github_actions.email}"
}
