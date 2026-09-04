# Investigation Report: form-submission-job Failure

**Incident ID:** `test-manual-check-v2`  
**Cloud Run Job:** `form-submission-job`  
**GCP Project:** `receipt-processing-egym`  
**Failed Bucket:** `receipt-processing-egym-receipts-failed`  
**Git Branch:** `investigate/job-failure-33905631401`  
**Issue:** [#31](https://github.com/5pirit5eal/wellpass-autoform/issues/31)  
**Compare / PR URL:** [main...investigate/job-failure-33905631401](https://github.com/5pirit5eal/wellpass-autoform/compare/main...investigate/job-failure-33905631401)

---

## 1. Executive Summary

During execution of Cloud Run job `form-submission-job` for batch `sub_2026-08_batch1_180855`, the container failed with exit code 1 at `2026-09-04 18:09:21 UTC`. The failure resulted from a timeout waiting for the welcome screen button (`"Beginnen"`) on Typeform.

Inspection of custom metadata and downloaded failure screenshots from `gs://receipt-processing-egym-receipts-failed/screenshots/` revealed that EGYM closed the existing Typeform endpoint (`https://egym.typeform.com/to/z5XBrNXf`) and migrated all submissions to a new form at `https://survey.egym.com/to/AwCQrDZ9`. Furthermore, the new form replaced the welcome button text (`"Beginnen"` -> `"Starten"`) and the final submission button (`"Senden"` / `"Antworten übermitteln"` -> `"Einschicken"`).

A complete fix was formulated, applied across the codebase, and verified end-to-end via Playwright live dry run and unit tests.

---

## 2. GCS Object Custom Metadata & Screenshot Evidence

Objects in `gs://receipt-processing-egym-receipts-failed/screenshots/2026-08/sub_2026-08_batch1_180855/` were inspected:

### Custom Metadata on `error_welcome_button.png`:
```json
{
  "batch_id": "sub_2026-08_batch1_180855",
  "error_message": "welcome button 'Beginnen' not found: playwright: timeout: Timeout 15000ms exceeded.",
  "failed_at": "2026-09-04T18:09:16Z",
  "month": "2026-08",
  "receipt_files": "8e3e1eae307d427480b57565c1b89cdb.pdf,94db0efbc27c4b31afc46438058628ca.pdf",
  "status": "failed",
  "uploaded_at": "2026-09-04T18:09:16Z"
}
```

### Visual Inspection of Screenshot:
The captured screenshot `sub_2026-08_batch1_180855_180916_error_welcome_button.png` showed EGYM's decommissioning message:
> **"Dieses Typeform-Formular lässt keine neuen Antworten zu.**  
> **Bitte nutze ab sofort das neue Formular um deine Bäderkarten einzureichen:**  
> **https://survey.egym.com/to/AwCQrDZ9"**

---

## 3. Cloud Logging Diagnostics

Cloud Run job log excerpt (`2026-09-04 18:08:55Z` - `2026-09-04 18:09:21Z`):
- `18:08:55Z`: Starting form submission job for submission month: 2026-08 (eligible receipt window: [2026-06 2026-07 2026-08], Bucket: receipt-processing-egym-receipts-processed, DryRun: false)
- `18:08:55Z`: Found 2 eligible receipt(s) to process for submission month 2026-08. Chunked 2 valid ticket(s) into 1 batch(es).
- `18:08:59Z`: `[sub_2026-08_batch1_180855] Navigating to Typeform: https://egym.typeform.com/to/z5XBrNXf`
- `18:09:16Z`: Uploading 2 screenshot(s) to inspection bucket `gs://receipt-processing-egym-receipts-failed/screenshots/2026-08/sub_2026-08_batch1_180855/...`
- `18:09:21Z`: `Job completed with error: submission failed for batch sub_2026-08_batch1_180855: welcome button 'Beginnen' not found: playwright: timeout: Timeout 15000ms exceeded.`
- `18:09:21Z`: Container called exit(1).

---

## 4. Root Cause Analysis

1. **Upstream Endpoint Deprecation:** EGYM migrated the reimbursement form from `https://egym.typeform.com/to/z5XBrNXf` to `https://survey.egym.com/to/AwCQrDZ9`. The old URL serves a static closure announcement without input controls or the `"Beginnen"` button.
2. **UI Selector Changes:**
   - **Start Button:** On the new form, the start button is `"Starten"` instead of `"Beginnen"`.
   - **Submit Button:** The final submission button on the confirmation step is `"Einschicken"` instead of `"Senden"` or `"Antworten übermitteln"`.

---

## 5. Applied Fix

The following changes were committed to `investigate/job-failure-33905631401`:

1. **`form-submission-job/internal/config/config.go` & `config_test.go`**:
   - Updated default `typeformURL` fallback to `"https://survey.egym.com/to/AwCQrDZ9"`.
   - Added unit test asserting default URL configuration.
2. **`form-submission-job/internal/submitter/playwright.go`**:
   - Updated welcome button selector to find `"Starten"` or `"Beginnen"` (`page.GetByRole("button", ...).Or(...)`).
   - Updated submit button selector to support `"Einschicken"`, `"Senden"`, and `"Antworten übermitteln"`.
3. **`form-submission-job/internal/submitter/integration_test.go`**:
   - Updated integration test URL to `"https://survey.egym.com/to/AwCQrDZ9"`.
4. **Environment & Deployment Configs**:
   - `terraform/github.tf`: Updated `WELLPASS_TYPEFORM_URL` repository variable default.
   - `form-submission-job/Taskfile.sh`, `deploy.sh`, `README.md`, `.env.example`: Updated default URLs.
5. **`.gitignore`**:
   - Added `gha-creds*.json` to prevent ephemeral runner tokens from being tracked.

---

## 6. Verification Results

1. **Unit Tests:**
   ```bash
   cd form-submission-job && go test -count=1 -v ./...
   ```
   **Result:** All packages passed (`bigquery`, `config`, `matcher`, `processor`, `storage`, `submitter`).

2. **Live Integration Dry Run Test:**
   ```bash
   go test -v -tags=integration ./internal/submitter -run TestPlaywrightLiveDryRun
   ```
   **Result:** PASSED in 61s against `https://survey.egym.com/to/AwCQrDZ9`.
   - Navigated to `https://survey.egym.com/to/AwCQrDZ9`
   - Successfully clicked `"Starten"`
   - Successfully dismissed notice screen (`"Weiter"`)
   - Filled member email
   - Successfully processed 3 tickets:
     - Schwimm' in Bilk Düsseldorf on 15.08.2026 (5,44 €)
     - Münster Therme Düsseldorf on 16.08.2026 (5,40 €)
     - Freizeitbad Düsselstrand Düsseldorf on 17.08.2026 (7,50 €)
   - Successfully uploaded receipts and advanced through more-tickets radio prompts
   - Filled personal information (Name, IBAN, BIC)
   - Confirmed declaration checkbox
   - Reached pre-submit summary screen with `"Einschicken"` button visible.

---

## 7. Next Steps & Recommendations

- **PR Creation Note:** GitHub repository setting `Settings -> Actions -> General -> Workflow permissions -> Allow GitHub Actions to create and approve pull requests` is currently disabled for this repository, preventing automated PR creation via `gh pr create`.
- Please merge branch `investigate/job-failure-33905631401` into `main` via:
  [https://github.com/5pirit5eal/wellpass-autoform/compare/main...investigate/job-failure-33905631401](https://github.com/5pirit5eal/wellpass-autoform/compare/main...investigate/job-failure-33905631401)
- Once merged, the GitHub Actions deployment workflow will build and deploy the updated container image and environment variables to Cloud Run.
