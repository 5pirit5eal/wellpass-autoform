resource "google_bigquery_dataset" "receipts" {
  dataset_id                  = var.bigquery_dataset_id
  friendly_name               = "Receipts Processing Data"
  description                 = "Dataset containing swimming pool receipts processing results for dashboards and automation"
  location                    = local.bq_location
  default_table_expiration_ms = null

  labels = {
    purpose = "receipts-analytics"
  }

  depends_on = [google_project_service.apis]
}

resource "google_bigquery_table" "processing_results" {
  dataset_id          = google_bigquery_dataset.receipts.dataset_id
  table_id            = "processing_results"
  description         = "Receipt processing outcomes, extracted metadata, Playwright form submissions, and audit trail"
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "processed_at"
  }

  clustering = ["status", "submission_status", "submission_month", "location"]

  schema = jsonencode([
    {
      name        = "receipt_id"
      type        = "STRING"
      mode        = "REQUIRED"
      description = "Unique identifier for the processed receipt record"
    },
    {
      name        = "source_filename"
      type        = "STRING"
      mode        = "REQUIRED"
      description = "Original filename of the receipt"
    },
    {
      name        = "date"
      type        = "DATE"
      mode        = "NULLABLE"
      description = "Extracted receipt date (YYYY-MM-DD)"
    },
    {
      name        = "ticket_price"
      type        = "NUMERIC"
      mode        = "NULLABLE"
      description = "Extracted ticket price amount in EUR"
    },
    {
      name        = "currency"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Currency code (e.g. EUR)"
    },
    {
      name        = "location"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Extracted swimming pool facility name"
    },
    {
      name        = "receipt_number"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Receipt, invoice, or booking reference number"
    },
    {
      name        = "customer_name"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Customer name listed on the receipt"
    },
    {
      name        = "ticket_type"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Type/description of the admission ticket"
    },
    {
      name        = "status"
      type        = "STRING"
      mode        = "REQUIRED"
      description = "Processing extraction status: processed, conflict, or failed"
    },
    {
      name        = "destination_bucket"
      type        = "STRING"
      mode        = "REQUIRED"
      description = "Bucket where the receipt was saved (target or failed)"
    },
    {
      name        = "conflict_reason"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Conflict explanation if status is conflict"
    },
    {
      name        = "error_message"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Error details if status is failed"
    },
    {
      name        = "raw_metadata"
      type        = "JSON"
      mode        = "NULLABLE"
      description = "Complete structured JSON metadata extracted by Gemini"
    },
    {
      name        = "processed_at"
      type        = "TIMESTAMP"
      mode        = "REQUIRED"
      description = "Timestamp when the receipt was processed"
    },
    {
      name        = "submission_status"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Submission status: pending, submitted, dry_run_success, unmatched_pool, or submission_failed"
    },
    {
      name        = "submission_month"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Target submission month in YYYY-MM format"
    },
    {
      name        = "batch_id"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Playwright submission batch identifier"
    },
    {
      name        = "matched_pool_label"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Fuzzy-matched swimming pool label in Typeform dropdown"
    },
    {
      name        = "matcher_score"
      type        = "FLOAT"
      mode        = "NULLABLE"
      description = "Confidence score of the pool matcher (0.0 to 1.0)"
    },
    {
      name        = "is_dry_run"
      type        = "BOOLEAN"
      mode        = "NULLABLE"
      description = "Whether the submission was executed as a dry run"
    },
    {
      name        = "submitted_at"
      type        = "TIMESTAMP"
      mode        = "NULLABLE"
      description = "Timestamp when the receipt was submitted to Typeform"
    },
    {
      name        = "archive_gcs_uri"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Permanent GCS archive URI in the submitted receipts bucket"
    },
    {
      name        = "screenshot_uris"
      type        = "STRING"
      mode        = "REPEATED"
      description = "List of GCS URIs for submission audit screenshots"
    },
    {
      name        = "submission_error"
      type        = "STRING"
      mode        = "NULLABLE"
      description = "Error message if form submission failed"
    },
    {
      name        = "last_updated_at"
      type        = "TIMESTAMP"
      mode        = "NULLABLE"
      description = "Timestamp of the last update to this record"
    }
  ])
}

# ---------------------------------------------------------------------------------------------------------------------
# Analytical Views
# ---------------------------------------------------------------------------------------------------------------------

resource "google_bigquery_table" "view_monthly_reimbursement_summary" {
  dataset_id          = google_bigquery_dataset.receipts.dataset_id
  table_id            = "v_monthly_reimbursement_summary"
  description         = "Monthly summary of receipts, total claimed vs. submitted reimbursement amounts, and status breakdown"
  deletion_protection = false

  view {
    query          = <<-SQL
      SELECT
        COALESCE(submission_month, FORMAT_DATE('%Y-%m', date)) AS month,
        COUNT(*) AS total_receipts,
        COUNTIF(status = 'processed') AS total_extracted_successfully,
        COUNTIF(submission_status = 'submitted') AS total_submitted,
        COUNTIF(submission_status = 'dry_run_success') AS total_dry_run_submitted,
        COUNTIF(submission_status = 'pending' OR (status = 'processed' AND submission_status IS NULL)) AS total_pending_submission,
        COUNTIF(status = 'failed' OR submission_status = 'submission_failed') AS total_failed,
        SUM(ticket_price) AS total_amount_claimed,
        SUM(CASE WHEN submission_status IN ('submitted', 'dry_run_success') THEN ticket_price ELSE 0 END) AS total_amount_submitted,
        AVG(ticket_price) AS avg_ticket_price
      FROM `${var.project_id}.${google_bigquery_dataset.receipts.dataset_id}.${google_bigquery_table.processing_results.table_id}`
      GROUP BY month
      ORDER BY month DESC
    SQL
    use_legacy_sql = false
  }

  depends_on = [google_bigquery_table.processing_results]
}

resource "google_bigquery_table" "view_pool_stats" {
  dataset_id          = google_bigquery_dataset.receipts.dataset_id
  table_id            = "v_pool_stats"
  description         = "Aggregated statistics per swimming pool facility"
  deletion_protection = false

  view {
    query          = <<-SQL
      SELECT
        COALESCE(matched_pool_label, location) AS pool_name,
        COUNT(*) AS total_visits,
        COUNTIF(submission_status IN ('submitted', 'dry_run_success')) AS submitted_visits,
        SUM(ticket_price) AS total_spent,
        AVG(ticket_price) AS avg_price_per_visit,
        MIN(date) AS first_visit_date,
        MAX(date) AS last_visit_date
      FROM `${var.project_id}.${google_bigquery_dataset.receipts.dataset_id}.${google_bigquery_table.processing_results.table_id}`
      WHERE status = 'processed'
      GROUP BY pool_name
      ORDER BY total_visits DESC, total_spent DESC
    SQL
    use_legacy_sql = false
  }

  depends_on = [google_bigquery_table.processing_results]
}

resource "google_bigquery_table" "view_pending_submissions" {
  dataset_id          = google_bigquery_dataset.receipts.dataset_id
  table_id            = "v_pending_submissions"
  description         = "Processed receipts that are currently pending Typeform submission"
  deletion_protection = false

  view {
    query          = <<-SQL
      SELECT
        receipt_id,
        source_filename,
        date,
        ticket_price,
        location,
        receipt_number,
        destination_bucket,
        processed_at
      FROM `${var.project_id}.${google_bigquery_dataset.receipts.dataset_id}.${google_bigquery_table.processing_results.table_id}`
      WHERE status = 'processed'
        AND (submission_status IS NULL OR submission_status = 'pending')
      ORDER BY date ASC, processed_at ASC
    SQL
    use_legacy_sql = false
  }

  depends_on = [google_bigquery_table.processing_results]
}
