#!/usr/bin/env bash
# ==============================================================================
# Deploy Cloud Run Job & Cloud Scheduler Trigger via gcloud CLI
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load .env if available
if [[ -f "${SCRIPT_DIR}/.env" ]]; then
  # shellcheck disable=SC1091
  set -a && source "${SCRIPT_DIR}/.env" && set +a
fi

# Load parent terraform outputs or variables if present
PROJECT_ID="${GCP_PROJECT_ID:-${PROJECT_ID:-$(gcloud config get-value project 2>/dev/null || echo "")}}"
REGION="${GCP_REGION:-${REGION:-"europe-west3"}}"
REPO_NAME="${ARTIFACT_REGISTRY_REPOSITORY:-${ARTIFACT_REGISTRY_REPO:-"golang"}}"
JOB_NAME="${JOB_NAME:-"form-submission-job"}"
SCHEDULER_JOB_NAME="${SCHEDULER_JOB_NAME:-"monthly-form-submission-trigger"}"
CRON_SCHEDULE="${CRON_SCHEDULE:-"0 8 2 * *"}"
TIME_ZONE="${TIME_ZONE:-"Europe/Berlin"}"

if [[ -z "${PROJECT_ID}" ]]; then
  echo "Error: PROJECT_ID or GCP_PROJECT_ID is required." >&2
  exit 1
fi

SOURCE_BUCKET="${SOURCE_BUCKET:-"${PROJECT_ID}-receipts-processed"}"
SUBMITTED_BUCKET="${SUBMITTED_BUCKET:-"${PROJECT_ID}-receipts-submitted"}"
FAILED_BUCKET="${FAILED_BUCKET:-"${PROJECT_ID}-receipts-failed"}"
TYPEFORM_URL="${TYPEFORM_URL:-"https://egym.typeform.com/to/z5XBrNXf"}"
EMAIL="${EMAIL:-""}"
FIRST_NAME="${FIRST_NAME:-""}"
LAST_NAME="${LAST_NAME:-""}"
IBAN="${IBAN:-""}"
BIC="${BIC:-""}"
DRY_RUN="${DRY_RUN:-"true"}"
JOB_SA="${SUBMISSION_JOB_SERVICE_ACCOUNT:-"form-submission-job-sa@${PROJECT_ID}.iam.gserviceaccount.com"}"
SCHEDULER_SA="${SCHEDULER_JOB_SERVICE_ACCOUNT:-"scheduler-job-invoker-sa@${PROJECT_ID}.iam.gserviceaccount.com"}"

IMAGE_TAG="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}/${JOB_NAME}:latest"

echo "===================================================================="
echo "Deploying Form Submission Job & Cloud Scheduler Trigger"
echo "===================================================================="
echo "Project:            ${PROJECT_ID}"
echo "Region:             ${REGION}"
echo "Image:              ${IMAGE_TAG}"
echo "Job Name:           ${JOB_NAME}"
echo "Scheduler Trigger:  ${SCHEDULER_JOB_NAME} (${CRON_SCHEDULE})"
echo "Job SA:             ${JOB_SA}"
echo "Scheduler SA:       ${SCHEDULER_SA}"
echo "Dry Run:            ${DRY_RUN}"
echo "===================================================================="

# 1. Build and push container image via Cloud Build
echo "==> Building and pushing container image via Google Cloud Build in ${REGION}..."
gcloud builds submit "${SCRIPT_DIR}" \
  --project="${PROJECT_ID}" \
  --region="${REGION}" \
  --default-buckets-behavior=regional-user-owned-bucket \
  --tag="${IMAGE_TAG}"

# 2. Deploy or update Cloud Run Job
echo "==> Deploying Cloud Run Job: ${JOB_NAME}..."
ENV_VARS="SOURCE_BUCKET=${SOURCE_BUCKET},SUBMITTED_BUCKET=${SUBMITTED_BUCKET},FAILED_BUCKET=${FAILED_BUCKET},TYPEFORM_URL=${TYPEFORM_URL},EMAIL=${EMAIL},FIRST_NAME=${FIRST_NAME},LAST_NAME=${LAST_NAME},IBAN=${IBAN},BIC=${BIC},DRY_RUN=${DRY_RUN},HEADLESS=true,SCREENSHOTS_DIR=/tmp/form-submission-screenshots"

if gcloud run jobs describe "${JOB_NAME}" --project="${PROJECT_ID}" --region="${REGION}" >/dev/null 2>&1; then
  echo "Updating existing Cloud Run Job ${JOB_NAME}..."
  gcloud run jobs update "${JOB_NAME}" \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --image="${IMAGE_TAG}" \
    --service-account="${JOB_SA}" \
    --memory="2Gi" \
    --cpu="1000m" \
    --max-retries=1 \
    --task-timeout="3600s" \
    --set-env-vars="${ENV_VARS}"
else
  echo "Creating new Cloud Run Job ${JOB_NAME}..."
  gcloud run jobs create "${JOB_NAME}" \
    --project="${PROJECT_ID}" \
    --region="${REGION}" \
    --image="${IMAGE_TAG}" \
    --service-account="${JOB_SA}" \
    --memory="2Gi" \
    --cpu="1000m" \
    --max-retries=1 \
    --task-timeout="3600s" \
    --set-env-vars="${ENV_VARS}"
fi

# 3. Deploy or update Cloud Scheduler trigger
echo "==> Deploying Cloud Scheduler Job: ${SCHEDULER_JOB_NAME}..."
JOB_RUN_URI="https://${REGION}-run.googleapis.com/v2/projects/${PROJECT_ID}/locations/${REGION}/jobs/${JOB_NAME}:run"

if gcloud scheduler jobs describe "${SCHEDULER_JOB_NAME}" --project="${PROJECT_ID}" --location="${REGION}" >/dev/null 2>&1; then
  echo "Updating existing Cloud Scheduler Job ${SCHEDULER_JOB_NAME}..."
  gcloud scheduler jobs update http "${SCHEDULER_JOB_NAME}" \
    --project="${PROJECT_ID}" \
    --location="${REGION}" \
    --schedule="${CRON_SCHEDULE}" \
    --time-zone="${TIME_ZONE}" \
    --uri="${JOB_RUN_URI}" \
    --http-method="POST" \
    --oauth-service-account-email="${SCHEDULER_SA}"
else
  echo "Creating new Cloud Scheduler Job ${SCHEDULER_JOB_NAME}..."
  gcloud scheduler jobs create http "${SCHEDULER_JOB_NAME}" \
    --project="${PROJECT_ID}" \
    --location="${REGION}" \
    --schedule="${CRON_SCHEDULE}" \
    --time-zone="${TIME_ZONE}" \
    --description="Triggers monthly Wellpass reimbursement form submission Cloud Run Job" \
    --uri="${JOB_RUN_URI}" \
    --http-method="POST" \
    --oauth-service-account-email="${SCHEDULER_SA}"
fi

echo "===================================================================="
echo "Deployment completed successfully!"
echo "To test run the Cloud Run Job immediately:"
echo "  gcloud run jobs execute ${JOB_NAME} --project=${PROJECT_ID} --region=${REGION} --wait"
echo "To trigger via Cloud Scheduler:"
echo "  gcloud scheduler jobs run ${SCHEDULER_JOB_NAME} --project=${PROJECT_ID} --location=${REGION}"
echo "===================================================================="
