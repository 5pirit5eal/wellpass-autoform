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
  description         = "Receipt processing outcomes, extracted metadata, and audit logs"
  deletion_protection = false

  time_partitioning {
    type  = "DAY"
    field = "processed_at"
  }

  clustering = ["status", "location"]

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
      description = "Processing status: processed, conflict, or failed"
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
    }
  ])
}
