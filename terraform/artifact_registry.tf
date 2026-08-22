resource "google_artifact_registry_repository" "receipts" {
  repository_id = var.artifact_registry_repo_id
  location      = var.region
  format        = "DOCKER"
  description   = "Docker container repository for receipts-function and submission jobs"

  labels = {
    purpose = "container-registry"
  }

  depends_on = [google_project_service.apis]
}
