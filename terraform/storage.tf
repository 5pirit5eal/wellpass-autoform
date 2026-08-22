# 1. Unprocessed Receipts Bucket (Source Bucket)
resource "google_storage_bucket" "unprocessed_receipts" {
  name                        = local.source_bucket
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  labels = {
    purpose = "unprocessed-receipts"
  }

  depends_on = [google_project_service.apis]
}

# 2. Processed Receipts Bucket (Archive Bucket)
resource "google_storage_bucket" "processed_receipts" {
  name                        = local.processed_bucket
  location                    = var.region
  storage_class               = "ARCHIVE"
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age = 3650 # Retain processed archives for 10 years
    }
  }

  labels = {
    purpose = "processed-receipts-archive"
  }

  depends_on = [google_project_service.apis]
}

# 3. Failed Receipts Bucket
resource "google_storage_bucket" "failed_receipts" {
  name                        = local.failed_bucket
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  labels = {
    purpose = "failed-receipts"
  }

  depends_on = [google_project_service.apis]
}

# Retrieve the Google Cloud Storage service agent for Eventarc notifications
data "google_storage_project_service_account" "gcs_account" {
  project = var.project_id
}

# Grant GCS service agent pubsub.publisher role to allow Cloud Storage trigger notifications
resource "google_project_iam_member" "gcs_pubsub_publisher" {
  project = var.project_id
  role    = "roles/pubsub.publisher"
  member  = "serviceAccount:${data.google_storage_project_service_account.gcs_account.email_address}"

  depends_on = [google_project_service.apis]
}
