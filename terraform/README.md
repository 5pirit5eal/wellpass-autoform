# Infrastructure via Terraform (IaC)

This directory provides the Terraform infrastructure as code (IaC) required for the **wellpass-autoform** platform, provisioning the core cloud foundation:

- **Google Cloud Storage Buckets (4-Bucket Storage Lifecycle)**:
  - `unprocessed_receipts` (`STANDARD`): Ingestion bucket where incoming receipts are uploaded to trigger processing.
  - `processed_receipts` (`STANDARD`): Transitional staging bucket for verified receipts with attached structured metadata.
  - `submitted_receipts` (`ARCHIVE`): Long-term archive bucket (10-year retention lifecycle rule) for receipts submitted to EGYM Wellpass.
  - `failed_receipts` (`COLDLINE`): Storage for rejected receipts, conflicts, and Playwright execution inspection screenshots.
- **IAM & Service Accounts**:
  - `receipts-function-sa`: Runtime service account for the Cloud Run function with permissions for Vertex AI Gemini, GCS, Cloud Logging, BigQuery, and Eventarc.
  - `form-submission-job-sa`: Runtime service account for the Cloud Run batch submission job with permissions across `processed`, `submitted`, and `failed` buckets.
  - `scheduler-job-invoker-sa`: Invoker service account used by Cloud Scheduler to trigger the Cloud Run submission job.
  - `gdrive-uploader-sa`: Service account for Google Drive automation scripts to upload incoming receipts to `unprocessed_receipts`.
  - `github-actions-sa`: Deployment service account used in CI/CD pipelines.
- **Workload Identity Federation (WIF)**:
  - Keyless authentication from GitHub Actions workflows to Google Cloud via OIDC.
- **BigQuery Analytics**:
  - Dataset (`receipts_processing`) and partitioned/clustered table (`processing_results`) for receipt extraction auditing and reporting.
- **Artifact Registry**:
  - Docker repository (`golang`) for container images.
- **GitHub Actions Configuration**:
  - Automated provisioning of GitHub repository variables and encrypted secrets (`WELLPASS_IBAN`) via the `integrations/github` provider.

---

## Architecture & Storage Lifecycle

```
[ Google Drive / Scanner ]
           |
           v (gdrive-uploader-sa)
+------------------------------------------------+
| 1. Unprocessed Bucket (STANDARD)               |
+------------------------------------------------+
           |
           v (CloudEvent: object.v1.finalized)
+------------------------------------------------+
| receipts-function (Cloud Run Function)         |
| - Extracts structured metadata via Gemini      |
| - Verifies duplicates & records in BigQuery   |
+------------------------------------------------+
      |                                  |
      | (Clean / Processed)              | (Extraction Failure / Conflict)
      v                                  v
+-----------------------------+    +-----------------------------+
| 2. Processed Bucket         |    | 4. Failed Bucket (COLDLINE) |
|    (STANDARD - Transitional)|    |    (Errors & Conflicts)     |
+-----------------------------+    +-----------------------------+
      |
      v (Monthly trigger: 2nd of each month via Cloud Scheduler)
+------------------------------------------------+
| form-submission-job (Cloud Run Job)            |
| - Discovers receipts within 3-month window     |
| - Resolves pools with fuzzy token matcher     |
| - Playwright automation on Typeform            |
| - Uploads audit screenshots to failed bucket   |
| - Deletes processed files once moved           |
+------------------------------------------------+
      |                                  |
      | (Submitted)                      | (Audit Screenshots / Failures)
      v                                  v
+-----------------------------+    +-----------------------------+
| 3. Submitted Bucket         |    | 4. Failed Bucket (COLDLINE) |
|    (ARCHIVE - 10yr retain)  |    |    (Screenshots & Logs)     |
|    Path: YYYY-MM/<filename> |    |    Path: screenshots/YYYY-MM|
+-----------------------------+    +-----------------------------+
```

---

## Managed Resources

### 1. Cloud Storage Buckets

| Bucket Identifier | Purpose | Storage Class | Versioning | Lifecycle Policy |
| --- | --- | --- | --- | --- |
| `unprocessed_receipts` | Ingestion for incoming tickets/receipts | `STANDARD` | Enabled | — |
| `processed_receipts` | Transitional staging for verified receipts | `STANDARD` | Enabled | Cleaned up on submission or deleted after 4 months (120 days) |
| `submitted_receipts` | Long-term archival of submitted receipts | `ARCHIVE` | Enabled | Delete after 10 years (3,650 days) |
| `failed_receipts` | Conflicts, unmatchable receipts, and audit screenshots | `COLDLINE` | Enabled | — |

*Note: Uniform bucket-level access is enforced across all buckets.*

### 2. Service Accounts & IAM Permissions

| Service Account | Role Bindings | Purpose |
| --- | --- | --- |
| `receipts-function-sa` | `roles/aiplatform.user`<br>`roles/storage.objectAdmin`<br>`roles/logging.logWriter`<br>`roles/bigquery.dataEditor`<br>`roles/bigquery.jobUser`<br>`roles/eventarc.eventReceiver` | Cloud Run Function runtime execution |
| `form-submission-job-sa` | `roles/storage.objectAdmin`<br>`roles/logging.logWriter` | Cloud Run Job batch submission execution |
| `scheduler-job-invoker-sa` | `roles/run.invoker` | Cloud Scheduler trigger invocation |
| `gdrive-uploader-sa` | `roles/storage.objectCreator` (on `unprocessed` bucket) | Google Drive sync script upload |
| `github-actions-sa` | `roles/cloudfunctions.developer`<br>`roles/run.developer`<br>`roles/cloudbuild.builds.editor`<br>`roles/cloudscheduler.admin`<br>`roles/artifactregistry.writer`<br>`roles/storage.admin`<br>`roles/eventarc.admin`<br>`roles/iam.serviceAccountUser` | GitHub Actions CI/CD deployment |

### 3. BigQuery Dataset & Table

- **Dataset**: `receipts_processing` (Location: `europe-west3` / `EU`)
- **Table**: `processing_results`
  - **Partitioning**: Day-partitioned on `processed_at` timestamp.
  - **Clustering**: Clustered on `["status", "location"]`.
  - **Schema**:
    - `receipt_id` (STRING, REQUIRED)
    - `source_filename` (STRING, REQUIRED)
    - `date` (DATE, NULLABLE)
    - `ticket_price` (NUMERIC, NULLABLE)
    - `currency` (STRING, NULLABLE)
    - `location` (STRING, NULLABLE)
    - `receipt_number` (STRING, NULLABLE)
    - `customer_name` (STRING, NULLABLE)
    - `ticket_type` (STRING, NULLABLE)
    - `status` (STRING, REQUIRED) — `processed`, `conflict`, or `failed`
    - `destination_bucket` (STRING, REQUIRED)
    - `conflict_reason` (STRING, NULLABLE)
    - `error_message` (STRING, NULLABLE)
    - `raw_metadata` (JSON, NULLABLE)
    - `processed_at` (TIMESTAMP, REQUIRED)

### 4. Workload Identity Federation (WIF)

- **Pool**: `github-actions-pool`
- **Provider**: `github-provider` (OIDC issuer: `https://token.actions.githubusercontent.com`)
- **Condition**: Restricted to repositories under the configured `github_owner`.

### 5. GitHub Repository Variables & Secrets

When `enable_github_resources = true`, the following repository variables and secrets are configured:

#### Variables (`vars.*`)

- `GCP_PROJECT_ID`
- `GCP_REGION`
- `GEMINI_LOCATION`
- `GEMINI_MODEL`
- `GOOGLE_GENAI_USE_VERTEXAI`
- `GCP_WORKLOAD_IDENTITY_PROVIDER`
- `GCP_SERVICE_ACCOUNT`
- `RECEIPTS_SOURCE_BUCKET`
- `RECEIPTS_TARGET_BUCKET`
- `RECEIPTS_SUBMITTED_BUCKET`
- `RECEIPTS_FAILED_BUCKET`
- `ARTIFACT_REGISTRY_REPOSITORY`
- `BIGQUERY_DATASET`
- `BIGQUERY_TABLE`
- `GDRIVE_UPLOADER_SERVICE_ACCOUNT`
- `SUBMISSION_JOB_SERVICE_ACCOUNT`
- `WELLPASS_TYPEFORM_URL`
- `WELLPASS_EMAIL`
- `WELLPASS_FIRST_NAME`
- `WELLPASS_LAST_NAME`
- `WELLPASS_BIC`
- `SUBMISSION_DRY_RUN`

#### Secrets (`secrets.*`)

- `WELLPASS_IBAN` (Sensitive banking information stored encrypted in GitHub Secrets)

---

## Deployment Guide

### Prerequisites

1. **Google Cloud SDK (`gcloud`)**: Authenticated with admin permissions on the target GCP project.
2. **Terraform CLI** (`>= 1.5.0`).
3. **GitHub Personal Access Token (PAT)**: Optional, required if configuring GitHub repository variables (`GITHUB_TOKEN` environment variable with `repo` permissions).

### Quick Start

1. **Configure Variables**:

   ```bash
   cp terraform.tfvars.example terraform.tfvars
   # Edit terraform.tfvars with your GCP project_id and member details
   ```

2. **Initialize Terraform**:

   ```bash
   terraform init
   ```

3. **Preview Infrastructure Plan**:

   ```bash
   terraform plan
   ```

4. **Apply Infrastructure**:

   ```bash
   terraform apply
   ```

5. **Verify Outputs**:

   ```bash
   terraform output
   ```

---

## Service Deployment (via `gcloud` / Taskfiles)

Terraform provisions all base cloud infrastructure. The application services are deployed using `gcloud` via their respective Taskfiles:

- **Cloud Run Function (`receipts-function`)**:

  ```bash
  cd ../receipts-function
  ./Taskfile.sh deploy
  ```

- **Cloud Run Job & Scheduler (`form-submission-job`)**:

  ```bash
  cd ../form-submission-job
  ./Taskfile.sh deploy
  ```
