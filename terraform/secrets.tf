# ---------------------------------------------------------------------------------------------------------------------
# Secret Manager: GitHub Dispatch Token (PAT) for Cloud Run Job Failure Webhook
# ---------------------------------------------------------------------------------------------------------------------
resource "google_secret_manager_secret" "github_dispatch_token" {
  secret_id = "github-dispatch-token"

  replication {
    auto {}
  }

  labels = {
    purpose = "github-actions-dispatch"
  }

  depends_on = [google_project_service.apis]
}

# Grant access to Failure Dispatcher Cloud Function service account
resource "google_secret_manager_secret_iam_member" "failure_dispatcher_token_accessor" {
  secret_id = google_secret_manager_secret.github_dispatch_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.failure_dispatcher.email}"
}

# Grant access to GitHub Actions deployment service account
resource "google_secret_manager_secret_iam_member" "github_actions_token_accessor" {
  secret_id = google_secret_manager_secret.github_dispatch_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.github_actions.email}"
}
