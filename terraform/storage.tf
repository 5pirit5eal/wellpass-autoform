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

# 2. Processed Receipts Bucket (Transitional Standard Bucket)
resource "google_storage_bucket" "processed_receipts" {
  name                        = local.processed_bucket
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age = 120 # Delete after 4 months (120 days) since EGYM policy permits submissions up to 3 months
    }
  }

  labels = {
    purpose = "processed-receipts-transitional"
  }

  depends_on = [google_project_service.apis]
}

# 3. Submitted Receipts Bucket (Archive Bucket with 10-year retention)
resource "google_storage_bucket" "submitted_receipts" {
  name                        = local.submitted_bucket
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
      age = 3650 # Retain submitted archives for 10 years (tax/audit compliance)
    }
  }

  labels = {
    purpose = "submitted-receipts-archive"
  }

  depends_on = [google_project_service.apis]
}

# 4. Failed / Rejected Receipts Bucket (Coldline Bucket)
resource "google_storage_bucket" "failed_receipts" {
  name                        = local.failed_bucket
  location                    = var.region
  storage_class               = "COLDLINE"
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  labels = {
    purpose = "failed-receipts-coldline"
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
