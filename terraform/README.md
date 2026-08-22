# Infrastructure via Terraform (IaC)

This directory provides the Terraform infrastructure as code (IaC) required for the **wellpass-autoform** platform, specifically provisioning:

- **Google Cloud Storage Buckets**:
  - `unprocessed_receipts` (Standard): Source bucket where incoming receipts are uploaded to trigger processing.
  - `processed_receipts` (Archive): Archive bucket for successfully processed receipts with attached structured metadata.
  - `failed_receipts` (Standard): Bucket for receipts that encountered conflicts or processing failures.
- **IAM & Service Accounts**:
  - `receipts-function-sa`: Runtime service account used by the Cloud Run function with least-privilege roles for Vertex AI (`roles/aiplatform.user`), Cloud Storage, Cloud Logging, BigQuery, and Eventarc.
  - `github-actions-sa`: Deployment service account used in CI/CD pipelines.
- **Workload Identity Federation (WIF)**:
  - Secure, keyless authentication from GitHub Actions workflows to Google Cloud without storing long-lived service account keys.
- **BigQuery Analytics**:
  - Dataset (`receipts_processing`) and partitioned/clustered table (`processing_results`) for receipt extraction outcomes, structured queries, and dashboard reporting.
- **Artifact Registry**:
  - Docker repository (`golang`) for container images.
- **GitHub Actions Configuration**:
  - Automated configuration of GitHub repository variables via the `integrations/github` provider.

---

## Architecture & Resource Map

```
                                      +-----------------------------------+
                                      |   GitHub Actions CI/CD Workflow   |
                                      +-----------------+-----------------+
                                                        |
                                                        v (WIF / OIDC)
                                      +-----------------+-----------------+
                                      |   github-actions-sa (Deploy SA)   |
                                      +-----------------+-----------------+
                                                        |
                                                        v
+------------------------+      CloudEvent      +-------+-------------------+
|  Unprocessed Bucket    | -------------------> |  receipts-function        |
|  (Source: PDF / TXT)   |                      |  (Go 1.26 + Gemini 3.5)   |
+------------------------+                      +---------+-------+---------+
                                                          |       |
                                   +----------------------+       +----------------------+
                                   | (Success)                                           | (Conflict/Failure)
                                   v                                                     v
                     +-------------+------------+                          +-------------+------------+
                     | Processed Archive Bucket |                          |  Failed Receipts Bucket  |
                     | (Attached GCS Metadata)  |                          |  (Attached Error / Reason)|
                     +--------------------------+                          +--------------------------+
                                   |                                                     |
                                   +----------------------+       +----------------------+
                                                          |       |
                                                          v       v
                                                +---------+-------+---------+
                                                |   BigQuery Analytics      |
                                                |   (processing_results)    |
                                                +---------------------------+
```

---

## Managed Resources

### 1. Cloud Storage Buckets

| Bucket Identifier | Purpose | Storage Class | Versioning | Lifecycle |
| --- | --- | --- | --- | --- |
| `unprocessed_receipts` | Source bucket for incoming tickets/receipts | `STANDARD` | Enabled | — |
| `processed_receipts` | Archive bucket for processed receipts | `ARCHIVE` | Enabled | Delete after 10 years |
| `failed_receipts` | Bucket for conflicts & failed extraction | `STANDARD` | Enabled | — |

*Note: Uniform bucket-level access is enforced across all buckets.*

### 2. BigQuery Dataset & Table

- **Dataset**: `receipts_processing` (Location: `europe-west3` / `EU`)
- **Table**: `processing_results`
  - **Partitioning**: Day-partitioned on `processed_at` timestamp.
  - **Clustering**: Clustered on `["status", "location"]` for fast queries and dashboard filters.
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

### 3. Service Accounts & IAM Permissions

#### Cloud Run Function Runtime (`receipts-function-sa`)

- `roles/aiplatform.user` (Invoke Vertex AI Gemini models)
- `roles/storage.objectAdmin` (Read source bucket, write to target/failed buckets)
- `roles/logging.logWriter` (Send logs to Cloud Logging)
- `roles/bigquery.dataEditor` & `roles/bigquery.jobUser` (Insert audit rows into BigQuery)
- `roles/eventarc.eventReceiver` (Receive Cloud Storage finalization events)

#### GitHub Actions Deployer (`github-actions-sa`)

- `roles/cloudfunctions.developer`
- `roles/run.developer`
- `roles/artifactregistry.writer`
- `roles/storage.admin`
- `roles/eventarc.admin`
- `roles/iam.serviceAccountUser` (on `receipts-function-sa`)

#### Google Drive Receipts Uploader (`gdrive-uploader-sa`)

- `roles/storage.objectCreator` (on the `unprocessed_receipts` source bucket)
- Used by Google Drive sync / automation scripts to upload incoming receipts to Cloud Storage.

### 4. Workload Identity Federation (WIF)

- **Pool**: `github-actions-pool`
- **Provider**: `github-provider` (OIDC issuer: `https://token.actions.githubusercontent.com`)
- **Condition**: Restricted to repositories under the configured `github_owner`.

### 5. GitHub Repository Variables

When `enable_github_resources = true`, the following repository variables are automatically configured in GitHub:

- `GCP_PROJECT_ID`
- `GCP_REGION`
- `GCP_WORKLOAD_IDENTITY_PROVIDER`
- `GCP_SERVICE_ACCOUNT`
- `RECEIPTS_SOURCE_BUCKET`
- `RECEIPTS_TARGET_BUCKET`
- `RECEIPTS_FAILED_BUCKET`
- `ARTIFACT_REGISTRY_REPOSITORY`
- `BIGQUERY_DATASET`
- `BIGQUERY_TABLE`
- `GDRIVE_UPLOADER_SERVICE_ACCOUNT`

---

## Deployment Guide

### Prerequisites

1. **Google Cloud SDK (`gcloud`)**: Authenticated with admin permissions on the target GCP project.
2. **Terraform CLI** (`>= 1.5.0`).
3. **GitHub Personal Access Token (PAT)**: Required if managing GitHub repository variables (`GITHUB_TOKEN` environment variable with `repo` permissions).

### Quick Start

1. **Configure Variables**:

   ```bash
   cp terraform.tfvars.example terraform.tfvars
   # Edit terraform.tfvars with your GCP project_id and preferences
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

## GitHub Actions CI/CD Integration

In your GitHub Actions workflow (`.github/workflows/deploy.yml`), you can authenticate using Workload Identity Federation directly with the variables created by Terraform:

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write

    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ vars.GCP_WORKLOAD_IDENTITY_PROVIDER }}
          service_account: ${{ vars.GCP_SERVICE_ACCOUNT }}

      - name: Set up Cloud SDK
        uses: google-github-actions/setup-gcloud@v2

      - name: Deploy Cloud Run Function
        run: |
          cd receipts-function
          ./Taskfile.sh deploy
```
