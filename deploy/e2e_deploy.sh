#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="open-calendar-sync"
REGION="${REGION:-europe-west1}"
REPO="calendarsync"
API_SERVICE="calendarsync-api"
WORKER_SERVICE="calendarsync-worker"
API_SA_ID="calendarsync-api"
WORKER_SA_ID="calendarsync-worker"
SCHEDULER_SA_ID="calendarsync-scheduler"
KMS_KEY_RING="${KMS_KEY_RING:-calendarsync}"
KMS_KEY_NAME="${KMS_KEY_NAME:-oauth-token-key}"
WEB_DOMAIN="${WEB_DOMAIN:-}"
FRONTEND_URL="${FRONTEND_URL:-}"

if [[ -f .env && -z "${NEON_DB:-}" ]]; then
  NEON_DB="$(sed -n 's/^NEON_DB=//p' .env)"
fi

if [[ -z "${NEON_DB:-}" ]]; then
  echo "NEON_DB must be set in environment or .env"
  exit 1
fi

if [[ -z "${GOOGLE_OAUTH_CLIENT_ID:-}" || -z "${GOOGLE_OAUTH_CLIENT_SECRET:-}" ]]; then
  echo "GOOGLE_OAUTH_CLIENT_ID and GOOGLE_OAUTH_CLIENT_SECRET are required"
  exit 1
fi

gcloud --project "${PROJECT_ID}" services enable \
  run.googleapis.com \
  cloudbuild.googleapis.com \
  artifactregistry.googleapis.com \
  secretmanager.googleapis.com \
  cloudscheduler.googleapis.com \
  iamcredentials.googleapis.com \
  cloudkms.googleapis.com

PROJECT_NUMBER="$(gcloud --project "${PROJECT_ID}" projects describe "${PROJECT_ID}" --format='value(projectNumber)')"
PROJECT_NUMBER="$(printf "%s" "${PROJECT_NUMBER}")"

service_account_email() {
  local account_id="$1"
  printf "%s@%s.iam.gserviceaccount.com" "${account_id}" "${PROJECT_ID}"
}

ensure_service_account() {
  local account_id="$1"
  local display_name="$2"
  local email
  email="$(service_account_email "${account_id}")"
  gcloud --project "${PROJECT_ID}" iam service-accounts describe "${email}" >/dev/null 2>&1 || \
    gcloud --project "${PROJECT_ID}" iam service-accounts create "${account_id}" --display-name "${display_name}"
}

grant_secret_accessor() {
  local secret_name="$1"
  local member="$2"
  gcloud --project "${PROJECT_ID}" secrets add-iam-policy-binding "${secret_name}" \
    --member "${member}" \
    --role "roles/secretmanager.secretAccessor" >/dev/null
}

grant_run_invoker() {
  local service_name="$1"
  local member="$2"
  gcloud --project "${PROJECT_ID}" run services add-iam-policy-binding "${service_name}" \
    --region "${REGION}" \
    --member "${member}" \
    --role "roles/run.invoker" >/dev/null
}

API_SA="$(service_account_email "${API_SA_ID}")"
WORKER_SA="$(service_account_email "${WORKER_SA_ID}")"
SCHEDULER_SA="$(service_account_email "${SCHEDULER_SA_ID}")"

ensure_service_account "${API_SA_ID}" "CalendarSync API"
ensure_service_account "${WORKER_SA_ID}" "CalendarSync Worker"
ensure_service_account "${SCHEDULER_SA_ID}" "CalendarSync Scheduler"

gcloud --project "${PROJECT_ID}" kms keyrings describe "${KMS_KEY_RING}" --location "${REGION}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" kms keyrings create "${KMS_KEY_RING}" --location "${REGION}"
gcloud --project "${PROJECT_ID}" kms keys describe "${KMS_KEY_NAME}" --location "${REGION}" --keyring "${KMS_KEY_RING}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" kms keys create "${KMS_KEY_NAME}" \
    --location "${REGION}" \
    --keyring "${KMS_KEY_RING}" \
    --purpose encryption
KMS_CRYPTO_KEY="projects/${PROJECT_ID}/locations/${REGION}/keyRings/${KMS_KEY_RING}/cryptoKeys/${KMS_KEY_NAME}"

gcloud --project "${PROJECT_ID}" kms keys add-iam-policy-binding "${KMS_KEY_NAME}" \
  --location "${REGION}" \
  --keyring "${KMS_KEY_RING}" \
  --member "serviceAccount:${API_SA}" \
  --role "roles/cloudkms.cryptoKeyEncrypterDecrypter" >/dev/null
gcloud --project "${PROJECT_ID}" kms keys add-iam-policy-binding "${KMS_KEY_NAME}" \
  --location "${REGION}" \
  --keyring "${KMS_KEY_RING}" \
  --member "serviceAccount:${WORKER_SA}" \
  --role "roles/cloudkms.cryptoKeyEncrypterDecrypter" >/dev/null

gcloud --project "${PROJECT_ID}" artifacts repositories describe "${REPO}" --location "${REGION}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" artifacts repositories create "${REPO}" --repository-format docker --location "${REGION}" --description "CalendarSync images"

API_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${API_SERVICE}:latest"
WORKER_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${WORKER_SERVICE}:latest"

ensure_secret() {
  local name="$1"
  local value="$2"
  gcloud --project "${PROJECT_ID}" secrets describe "${name}" >/dev/null 2>&1 || \
    gcloud --project "${PROJECT_ID}" secrets create "${name}" --replication-policy automatic
  printf "%s" "${value}" | gcloud --project "${PROJECT_ID}" secrets versions add "${name}" --data-file=-
}

SCHEDULER_SECRET="$(openssl rand -hex 24)"
STATE_SECRET_B64="$(openssl rand -base64 32 | tr -d '\n')"
STATIC_ENCRYPTION_KEY_B64="$(openssl rand -base64 32 | tr -d '\n')"
FRONTEND_SHARED_SECRET="${FRONTEND_SHARED_SECRET:-$(openssl rand -hex 24)}"

ensure_secret "calendarsync-neon-db" "${NEON_DB}"
ensure_secret "calendarsync-google-client-id" "${GOOGLE_OAUTH_CLIENT_ID}"
ensure_secret "calendarsync-google-client-secret" "${GOOGLE_OAUTH_CLIENT_SECRET}"
ensure_secret "calendarsync-scheduler-secret" "${SCHEDULER_SECRET}"
ensure_secret "calendarsync-oauth-state-secret-b64" "${STATE_SECRET_B64}"
ensure_secret "calendarsync-static-encryption-key-b64" "${STATIC_ENCRYPTION_KEY_B64}"
ensure_secret "calendarsync-frontend-shared-secret" "${FRONTEND_SHARED_SECRET}"

grant_secret_accessor "calendarsync-neon-db" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-google-client-id" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-google-client-secret" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-scheduler-secret" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-oauth-state-secret-b64" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-static-encryption-key-b64" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-frontend-shared-secret" "serviceAccount:${API_SA}"

grant_secret_accessor "calendarsync-neon-db" "serviceAccount:${WORKER_SA}"
grant_secret_accessor "calendarsync-google-client-id" "serviceAccount:${WORKER_SA}"
grant_secret_accessor "calendarsync-google-client-secret" "serviceAccount:${WORKER_SA}"
grant_secret_accessor "calendarsync-scheduler-secret" "serviceAccount:${WORKER_SA}"
grant_secret_accessor "calendarsync-static-encryption-key-b64" "serviceAccount:${WORKER_SA}"

gcloud --project "${PROJECT_ID}" builds submit \
  --config deploy/cloudbuild.api.yaml \
  --substitutions "_IMAGE=${API_IMAGE}" .

gcloud --project "${PROJECT_ID}" builds submit \
  --config deploy/cloudbuild.worker.yaml \
  --substitutions "_IMAGE=${WORKER_IMAGE}" .
if [[ -z "${FRONTEND_URL}" && -n "${WEB_DOMAIN}" ]]; then
  FRONTEND_URL="https://${WEB_DOMAIN}"
fi
if [[ -n "${FRONTEND_URL}" ]]; then
  FRONTEND_URL="${FRONTEND_URL%/}"
fi
API_REDIRECT_URL="https://placeholder.invalid/oauth/google/callback"
if [[ -n "${FRONTEND_URL}" ]]; then
  API_REDIRECT_URL="${FRONTEND_URL}/oauth/google/callback"
fi

gcloud --project "${PROJECT_ID}" run deploy "${API_SERVICE}" \
  --region "${REGION}" \
  --image "${API_IMAGE}" \
  --allow-unauthenticated \
  --service-account "${API_SA}" \
  --set-secrets "DATABASE_URL=calendarsync-neon-db:latest,GOOGLE_OAUTH_CLIENT_ID=calendarsync-google-client-id:latest,GOOGLE_OAUTH_CLIENT_SECRET=calendarsync-google-client-secret:latest,SCHEDULER_SHARED_SECRET=calendarsync-scheduler-secret:latest,OAUTH_STATE_SECRET_B64=calendarsync-oauth-state-secret-b64:latest,CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64=calendarsync-static-encryption-key-b64:latest,FRONTEND_SHARED_SECRET=calendarsync-frontend-shared-secret:latest" \
  --set-env-vars "GOOGLE_OAUTH_REDIRECT_URL=${API_REDIRECT_URL},DISPATCH_MIN_INTERVAL_SECONDS=30,KMS_CRYPTO_KEY=${KMS_CRYPTO_KEY}"

grant_run_invoker "${API_SERVICE}" "serviceAccount:${SCHEDULER_SA}"

API_URL="$(gcloud --project "${PROJECT_ID}" run services describe "${API_SERVICE}" --region "${REGION}" --format='value(status.url)')"

gcloud --project "${PROJECT_ID}" run deploy "${WORKER_SERVICE}" \
  --region "${REGION}" \
  --image "${WORKER_IMAGE}" \
  --execution-environment gen1 \
  --cpu 0.25 \
  --memory 256Mi \
  --cpu-throttling \
  --min-instances 0 \
  --max-instances 1 \
  --concurrency 1 \
  --no-allow-unauthenticated \
  --service-account "${WORKER_SA}" \
  --set-secrets "DATABASE_URL=calendarsync-neon-db:latest,GOOGLE_OAUTH_CLIENT_ID=calendarsync-google-client-id:latest,GOOGLE_OAUTH_CLIENT_SECRET=calendarsync-google-client-secret:latest,SCHEDULER_SHARED_SECRET=calendarsync-scheduler-secret:latest,CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64=calendarsync-static-encryption-key-b64:latest" \
  --set-env-vars "KMS_CRYPTO_KEY=${KMS_CRYPTO_KEY}"
grant_run_invoker "${WORKER_SERVICE}" "serviceAccount:${SCHEDULER_SA}"
grant_run_invoker "${WORKER_SERVICE}" "serviceAccount:${API_SA}"

# Scheduler -> dispatch endpoint
gcloud --project "${PROJECT_ID}" scheduler jobs describe calendarsync-dispatch >/dev/null 2>&1 && \
  gcloud --project "${PROJECT_ID}" scheduler jobs delete calendarsync-dispatch --quiet
gcloud --project "${PROJECT_ID}" scheduler jobs create http calendarsync-dispatch \
  --location "${REGION}" \
  --schedule "*/10 * * * *" \
  --uri "${API_URL}/internal/scheduler/dispatch" \
  --http-method POST \
  --oidc-service-account-email "${SCHEDULER_SA}" \
  --oidc-token-audience "${API_URL}" \
  --headers "X-Scheduler-Secret=${SCHEDULER_SECRET}"

# Scheduler -> run worker service
WORKER_URL="$(gcloud --project "${PROJECT_ID}" run services describe "${WORKER_SERVICE}" --region "${REGION}" --format='value(status.url)')"
gcloud --project "${PROJECT_ID}" run services update "${API_SERVICE}" \
  --region "${REGION}" \
  --update-env-vars "WORKER_RUN_URL=${WORKER_URL}/internal/worker/run"
gcloud --project "${PROJECT_ID}" scheduler jobs describe calendarsync-run-worker >/dev/null 2>&1 && \
  gcloud --project "${PROJECT_ID}" scheduler jobs delete calendarsync-run-worker --quiet
gcloud --project "${PROJECT_ID}" scheduler jobs create http calendarsync-run-worker \
  --location "${REGION}" \
  --schedule "*/10 * * * *" \
  --uri "${WORKER_URL}/internal/worker/run" \
  --http-method POST \
  --oidc-service-account-email "${SCHEDULER_SA}" \
  --oidc-token-audience "${WORKER_URL}" \
  --headers "X-Scheduler-Secret=${SCHEDULER_SECRET}"
if gcloud --project "${PROJECT_ID}" run jobs describe "${WORKER_SERVICE}" --region "${REGION}" >/dev/null 2>&1; then
  gcloud --project "${PROJECT_ID}" run jobs delete "${WORKER_SERVICE}" --region "${REGION}" --quiet
fi

echo "Deployment completed."
echo "API URL: ${API_URL}"
echo "Google OAuth redirect URL configured in API: ${API_REDIRECT_URL}"
echo "Vercel frontend root directory: web"
if [[ -n "${FRONTEND_URL}" ]]; then
  echo "Frontend URL: ${FRONTEND_URL}"
else
  echo "Frontend URL not set. Redeploy the API with FRONTEND_URL once your Vercel URL or custom domain is known."
fi
echo "Configure these Vercel environment variables:"
echo "  AUTH_SECRET=${STATE_SECRET_B64}"
echo "  AUTH_TRUST_HOST=true"
echo "  AUTH_GOOGLE_ID=${GOOGLE_OAUTH_CLIENT_ID}"
echo "  AUTH_GOOGLE_SECRET=${GOOGLE_OAUTH_CLIENT_SECRET}"
echo "  CALENDARSYNC_API_URL=${API_URL}"
echo "  CALENDARSYNC_API_SHARED_SECRET=${FRONTEND_SHARED_SECRET}"
if [[ -n "${FRONTEND_URL}" ]]; then
  echo "  AUTH_URL=${FRONTEND_URL}"
  echo "Remember to add these redirect URLs to your Google OAuth client:"
  echo "  ${API_REDIRECT_URL}"
  echo "  ${FRONTEND_URL}/api/auth/callback/google"
  echo "Authorized JavaScript origin:"
  echo "  ${FRONTEND_URL}"
fi
