# ---------------------------------------------------------------------------------------------------------------------
# Cloud Run Job Failure Alerting & Pub/Sub Notification Channel
# ---------------------------------------------------------------------------------------------------------------------

# 1. Pub/Sub Topic for Cloud Run Job Failure Events
resource "google_pubsub_topic" "job_failures" {
  name = "cloud-run-job-failures"

  labels = {
    purpose = "cloud-run-job-failure-alert"
  }

  depends_on = [google_project_service.apis]
}

# 2. Allow Cloud Monitoring Notification Service Agent to publish alerts to Pub/Sub topic
resource "google_pubsub_topic_iam_member" "monitoring_publisher" {
  topic  = google_pubsub_topic.job_failures.name
  role   = "roles/pubsub.publisher"
  member = "serviceAccount:service-${data.google_project.current.number}@gcp-sa-monitoring-notification.iam.gserviceaccount.com"

  depends_on = [google_pubsub_topic.job_failures]
}

# 3. Cloud Monitoring Notification Channel (Pub/Sub)
resource "google_monitoring_notification_channel" "pubsub_failures" {
  display_name = "Cloud Run Job Failures Pub/Sub Channel"
  type         = "pubsub"

  labels = {
    topic = google_pubsub_topic.job_failures.id
  }

  description = "Pub/Sub notification channel for dispatching Cloud Run Job failures to GitHub Actions"

  depends_on = [
    google_pubsub_topic.job_failures,
    google_pubsub_topic_iam_member.monitoring_publisher
  ]
}

# 4. Cloud Monitoring Alert Policy: Trigger when form-submission-job execution fails
resource "google_monitoring_alert_policy" "job_failure" {
  display_name = "Cloud Run Job form-submission-job Execution Failure"
  combiner     = "OR"

  conditions {
    display_name = "Cloud Run Job Failed Execution Count > 0"

    condition_threshold {
      filter          = "resource.type = \"cloud_run_job\" AND metric.type = \"run.googleapis.com/job/completed_execution_count\" AND metric.labels.status = \"failed\" AND resource.labels.job_name = \"form-submission-job\""
      duration        = "0s"
      comparison      = "COMPARISON_GT"
      threshold_value = 0

      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.job_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = [
    google_monitoring_notification_channel.pubsub_failures.name
  ]

  documentation {
    content   = "The Cloud Run Job `form-submission-job` completed with a failed status. A notification has been sent via Pub/Sub to trigger automated investigation by OpenCode via GitHub Actions."
    mime_type = "text/markdown"
  }

  alert_strategy {
    auto_close = "1800s" # Auto close incident after 30 minutes
  }

  depends_on = [
    google_project_service.apis,
    google_monitoring_notification_channel.pubsub_failures
  ]
}
