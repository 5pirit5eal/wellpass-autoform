# Wellpass Autoform

Automated swimming pool receipt processing, metadata extraction, and monthly reimbursement submission for **EGYM Wellpass**.

---

## Overview

EGYM Wellpass allows members to submit out-of-pocket receipts from participating swimming pools for monthly reimbursement via Typeform. **Wellpass Autoform** fully automates this lifecycle:

1. **Receipt Ingestion**: Uploads incoming PDF/TXT receipts from Google Drive or scanner sync to Google Cloud Storage.
2. **AI Extraction & Auditing (`receipts-function`)**: Cloud Run Function triggered by CloudEvents using **Gemini 3.5 Flash Lite** to extract structured metadata (date, amount, pool facility, receipt ID), verify uniqueness, write audit rows to BigQuery, and stage valid items.
3. **Automated Submission (`form-submission-job`)**: Scheduled Cloud Run Job running headless **Playwright** automation with a 423-pool fuzzy matcher to resolve facility names, chunk tickets into batches, populate the Typeform reimbursement workflow, upload receipt attachments, capture audit screenshots, and archive files.
4. **4-Bucket Lifecycle**: Enforces strict data governance across ingestion, staging, long-term 10-year archival, and error tracking.

---

## Architecture & Data Flow

```
+-----------------------------------------------------------------------------------------+
|                                    1. INGESTION                                         |
|  Google Drive / Scanner Sync  --->  gs://<project>-receipts-unprocessed (STANDARD)      |
+-----------------------------------------------------------------------------------------+
                                                  |
                                                  v (CloudEvent: object.v1.finalized)
+-----------------------------------------------------------------------------------------+
|                               2. EXTRACTION & STAGING                                   |
|  receipts-function (Go 1.26 + Gemini 3.5 Flash Lite)                                    |
|  - Extracts date, price, location, receipt ID via structured JSON schema                |
|  - Validates duplicate avoidance & records outcome in BigQuery                          |
+-----------------------------------------------------------------------------------------+
           |                                                      |
           | (Extraction Success)                                 | (Extraction Conflict/Error)
           v                                                      v
  gs://<project>-receipts-processed (STANDARD)          gs://<project>-receipts-failed (COLDLINE)
  (Transitional staging with metadata)
           |
           v (Monthly Trigger: 2nd of each month via Cloud Scheduler)
+-----------------------------------------------------------------------------------------+
|                               3. BATCH SUBMISSION                                       |
|  form-submission-job (Go 1.26 + Playwright)                                             |
|  - Discovers receipts within eligible 3-month window                                    |
|  - Resolves facility names via 423-pool fuzzy token matcher                             |
|  - Automates Typeform form entry, file upload, & banking details                         |
|  - Captures inspection screenshots per stage                                            |
|  - Deletes processed source files once moved                                            |
+-----------------------------------------------------------------------------------------+
           |                                                      |
           | (Submitted Successfully)                             | (Audit Screenshots / Failures)
           v                                                      v
  gs://<project>-receipts-submitted (ARCHIVE)           gs://<project>-receipts-failed (COLDLINE)
  Path: YYYY-MM/<filename>                              Path: screenshots/YYYY-MM/<batch_id>/...
  (10-Year Lifecycle Retention)
```

---

## 4-Bucket Storage Lifecycle

| Bucket | Storage Class | Lifecycle / Retention | Purpose |
| --- | --- | --- | --- |
| **`unprocessed`** | `STANDARD` | Deleted upon processing | Landing bucket for raw receipt uploads. |
| **`processed`** | `STANDARD` | Deleted upon submission / Expired after 4 months (120 days) | Transitional staging with attached GCS object metadata. |
| **`submitted`** | `ARCHIVE` | 10 years (3,650 days) | Long-term compliance archive organized by submission month (`YYYY-MM/`). |
| **`failed`** | `COLDLINE` | Persistent | Storage for rejected receipts, conflicts, and Playwright audit screenshots. |

---

## Repository Modules & Sub-READMEs

| Module | Description | Documentation |
| --- | --- | --- |
| **`receipts-function/`** | Event-driven Cloud Run Function for Gemini-powered receipt extraction and BigQuery auditing. | [Read README](./receipts-function/README.md) |
| **`form-submission-job/`** | Monthly batch job automating EGYM Wellpass Typeform submission via Playwright. | [Read README](./form-submission-job/README.md) |
| **`terraform/`** | Infrastructure as Code (IaC) for GCS buckets, IAM service accounts, BigQuery, Artifact Registry, WIF, and GitHub secrets. | [Read README](./terraform/README.md) |
| **`examples/`** | Sample swimming pool receipts and tickets used for local testing and fixture verification. | [Browse Examples](./examples) |

---

## CI/CD & Automation

The repository includes pre-configured GitHub Actions workflows and automated dependency maintenance:

- **Validation Workflow (`.github/workflows/validate.yml`)**: Runs on pull requests and pushes to `main`. Executes `go vet`, `golangci-lint`, unit test suites with code coverage, and compiles binaries across both Go services.
- **Deployment Workflow (`.github/workflows/deploy.yml`)**: Uses Workload Identity Federation (WIF) for keyless deployment to Google Cloud. Deploys `receipts-function` (Cloud Run Function) and `form-submission-job` (Cloud Run Job & Cloud Scheduler trigger) via `gcloud` using their respective `Taskfile.sh` scripts.
- **Dependabot (`.github/dependabot.yml`)**: Automated weekly dependency version updates for GitHub Actions, Go modules (`receipts-function` and `form-submission-job`), and Docker configurations.

---

## Local Development Quickstart

Both Go services provide a standalone `Taskfile.sh` for development and testing:

### 1. Receipts Extraction Function

```bash
cd receipts-function
./Taskfile.sh test        # Run unit tests
./Taskfile.sh validate    # Run linting and static analysis
./Taskfile.sh run-local   # Start local CloudEvent server
```

### 2. Form Submission Job

```bash
cd form-submission-job
./Taskfile.sh test        # Run unit tests
./Taskfile.sh validate    # Run linting and static analysis
./Taskfile.sh run         # Execute local dry-run against Typeform
```

### 3. Cloud Infrastructure

```bash
cd terraform
terraform init
terraform plan
terraform apply
```

---

## Safety & Compliance

- **Dry-Run Mode**: `DRY_RUN=true` is enabled by default to prevent unintended submissions during testing.
- **Anonymized Testing**: All test fixtures and mock datasets use synthetic placeholder data.
- **Secret Management**: Banking details (`WELLPASS_IBAN`) are encrypted using GitHub Repository Secrets and injected securely at runtime.
