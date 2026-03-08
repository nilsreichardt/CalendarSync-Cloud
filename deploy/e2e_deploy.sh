#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="open-calendar-sync"
REGION="${REGION:-europe-west1}"
REPO="calendarsync"
API_SERVICE="calendarsync-api"
WEB_SERVICE="calendarsync-web"
WORKER_JOB="calendarsync-worker"
API_SA_ID="calendarsync-api"
WEB_SA_ID="calendarsync-web"
WORKER_SA_ID="calendarsync-worker"
SCHEDULER_SA_ID="calendarsync-scheduler"
KMS_KEY_RING="${KMS_KEY_RING:-calendarsync}"
KMS_KEY_NAME="${KMS_KEY_NAME:-oauth-token-key}"
WEB_DOMAIN="${WEB_DOMAIN:-}"

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

grant_service_account_user() {
  local service_account_email="$1"
  local member="$2"
  gcloud --project "${PROJECT_ID}" iam service-accounts add-iam-policy-binding "${service_account_email}" \
    --member "${member}" \
    --role "roles/iam.serviceAccountUser" >/dev/null
}

grant_run_invoker() {
  local service_name="$1"
  local member="$2"
  gcloud --project "${PROJECT_ID}" run services add-iam-policy-binding "${service_name}" \
    --region "${REGION}" \
    --member "${member}" \
    --role "roles/run.invoker" >/dev/null
}

remove_public_invoker() {
  local service_name="$1"
  gcloud --project "${PROJECT_ID}" run services remove-iam-policy-binding "${service_name}" \
    --region "${REGION}" \
    --member "allUsers" \
    --role "roles/run.invoker" >/dev/null 2>&1 || true
}

API_SA="$(service_account_email "${API_SA_ID}")"
WEB_SA="$(service_account_email "${WEB_SA_ID}")"
WORKER_SA="$(service_account_email "${WORKER_SA_ID}")"
SCHEDULER_SA="$(service_account_email "${SCHEDULER_SA_ID}")"

ensure_service_account "${API_SA_ID}" "CalendarSync API"
ensure_service_account "${WEB_SA_ID}" "CalendarSync Web"
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
WEB_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${WEB_SERVICE}:latest"
WORKER_IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${WORKER_JOB}:latest"

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

ensure_secret "calendarsync-neon-db" "${NEON_DB}"
ensure_secret "calendarsync-google-client-id" "${GOOGLE_OAUTH_CLIENT_ID}"
ensure_secret "calendarsync-google-client-secret" "${GOOGLE_OAUTH_CLIENT_SECRET}"
ensure_secret "calendarsync-scheduler-secret" "${SCHEDULER_SECRET}"
ensure_secret "calendarsync-oauth-state-secret-b64" "${STATE_SECRET_B64}"
ensure_secret "calendarsync-static-encryption-key-b64" "${STATIC_ENCRYPTION_KEY_B64}"

grant_secret_accessor "calendarsync-neon-db" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-google-client-id" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-google-client-secret" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-scheduler-secret" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-oauth-state-secret-b64" "serviceAccount:${API_SA}"
grant_secret_accessor "calendarsync-static-encryption-key-b64" "serviceAccount:${API_SA}"

grant_secret_accessor "calendarsync-oauth-state-secret-b64" "serviceAccount:${WEB_SA}"
grant_secret_accessor "calendarsync-google-client-id" "serviceAccount:${WEB_SA}"
grant_secret_accessor "calendarsync-google-client-secret" "serviceAccount:${WEB_SA}"

grant_secret_accessor "calendarsync-neon-db" "serviceAccount:${WORKER_SA}"
grant_secret_accessor "calendarsync-google-client-id" "serviceAccount:${WORKER_SA}"
grant_secret_accessor "calendarsync-google-client-secret" "serviceAccount:${WORKER_SA}"
grant_secret_accessor "calendarsync-static-encryption-key-b64" "serviceAccount:${WORKER_SA}"

gcloud --project "${PROJECT_ID}" builds submit \
  --config deploy/cloudbuild.api.yaml \
  --substitutions "_IMAGE=${API_IMAGE}" .

gcloud --project "${PROJECT_ID}" builds submit \
  --config deploy/cloudbuild.worker.yaml \
  --substitutions "_IMAGE=${WORKER_IMAGE}" .

gcloud --project "${PROJECT_ID}" builds submit \
  --config deploy/cloudbuild.web.yaml \
  --substitutions "_IMAGE=${WEB_IMAGE}" .

# First deploy web so we can compute redirect URL.
INITIAL_AUTH_URL="https://placeholder.invalid"
if [[ -n "${WEB_DOMAIN}" ]]; then
  INITIAL_AUTH_URL="https://${WEB_DOMAIN}"
fi
gcloud --project "${PROJECT_ID}" run deploy "${WEB_SERVICE}" \
  --region "${REGION}" \
  --image "${WEB_IMAGE}" \
  --allow-unauthenticated \
  --service-account "${WEB_SA}" \
  --set-env-vars "AUTH_TRUST_HOST=true,AUTH_URL=${INITIAL_AUTH_URL},CALENDARSYNC_API_URL=https://placeholder.invalid" \
  --set-secrets "AUTH_SECRET=calendarsync-oauth-state-secret-b64:latest,AUTH_GOOGLE_ID=calendarsync-google-client-id:latest,AUTH_GOOGLE_SECRET=calendarsync-google-client-secret:latest"

WEB_URL="$(gcloud --project "${PROJECT_ID}" run services describe "${WEB_SERVICE}" --region "${REGION}" --format='value(status.url)')"
PUBLIC_WEB_URL="${WEB_URL}"
if [[ -n "${WEB_DOMAIN}" ]]; then
  PUBLIC_WEB_URL="https://${WEB_DOMAIN}"
fi
API_REDIRECT_URL="${PUBLIC_WEB_URL}/oauth/google/callback"

gcloud --project "${PROJECT_ID}" run deploy "${API_SERVICE}" \
  --region "${REGION}" \
  --image "${API_IMAGE}" \
  --no-allow-unauthenticated \
  --service-account "${API_SA}" \
  --set-secrets "DATABASE_URL=calendarsync-neon-db:latest,GOOGLE_OAUTH_CLIENT_ID=calendarsync-google-client-id:latest,GOOGLE_OAUTH_CLIENT_SECRET=calendarsync-google-client-secret:latest,SCHEDULER_SHARED_SECRET=calendarsync-scheduler-secret:latest,OAUTH_STATE_SECRET_B64=calendarsync-oauth-state-secret-b64:latest,CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64=calendarsync-static-encryption-key-b64:latest" \
  --set-env-vars "GOOGLE_OAUTH_REDIRECT_URL=${API_REDIRECT_URL},DISPATCH_MIN_INTERVAL_SECONDS=30,KMS_CRYPTO_KEY=${KMS_CRYPTO_KEY}"

grant_run_invoker "${API_SERVICE}" "serviceAccount:${WEB_SA}"
grant_run_invoker "${API_SERVICE}" "serviceAccount:${SCHEDULER_SA}"
remove_public_invoker "${API_SERVICE}"

API_URL="$(gcloud --project "${PROJECT_ID}" run services describe "${API_SERVICE}" --region "${REGION}" --format='value(status.url)')"

gcloud --project "${PROJECT_ID}" run deploy "${WEB_SERVICE}" \
  --region "${REGION}" \
  --image "${WEB_IMAGE}" \
  --allow-unauthenticated \
  --service-account "${WEB_SA}" \
  --set-env-vars "AUTH_TRUST_HOST=true,AUTH_URL=${PUBLIC_WEB_URL},CALENDARSYNC_API_URL=${API_URL}" \
  --set-secrets "AUTH_SECRET=calendarsync-oauth-state-secret-b64:latest,AUTH_GOOGLE_ID=calendarsync-google-client-id:latest,AUTH_GOOGLE_SECRET=calendarsync-google-client-secret:latest"

if [[ -n "${WEB_DOMAIN}" ]]; then
  if gcloud --project "${PROJECT_ID}" beta run domain-mappings describe \
    --domain "${WEB_DOMAIN}" --region "${REGION}" >/dev/null 2>&1; then
    echo "Cloud Run domain mapping already exists for ${WEB_DOMAIN}"
  else
    gcloud --project "${PROJECT_ID}" beta run domain-mappings create \
      --service "${WEB_SERVICE}" \
      --domain "${WEB_DOMAIN}" \
      --region "${REGION}"
  fi
fi

gcloud --project "${PROJECT_ID}" run jobs deploy "${WORKER_JOB}" \
  --region "${REGION}" \
  --image "${WORKER_IMAGE}" \
  --service-account "${WORKER_SA}" \
  --set-secrets "DATABASE_URL=calendarsync-neon-db:latest,GOOGLE_OAUTH_CLIENT_ID=calendarsync-google-client-id:latest,GOOGLE_OAUTH_CLIENT_SECRET=calendarsync-google-client-secret:latest,CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64=calendarsync-static-encryption-key-b64:latest" \
  --set-env-vars "KMS_CRYPTO_KEY=${KMS_CRYPTO_KEY}"

# Scheduler -> dispatch endpoint
gcloud --project "${PROJECT_ID}" scheduler jobs describe calendarsync-dispatch >/dev/null 2>&1 && \
  gcloud --project "${PROJECT_ID}" scheduler jobs delete calendarsync-dispatch --quiet
gcloud --project "${PROJECT_ID}" scheduler jobs create http calendarsync-dispatch \
  --location "${REGION}" \
  --schedule "* * * * *" \
  --uri "${API_URL}/internal/scheduler/dispatch" \
  --http-method POST \
  --oidc-service-account-email "${SCHEDULER_SA}" \
  --oidc-token-audience "${API_URL}" \
  --headers "X-Scheduler-Secret=${SCHEDULER_SECRET}"

# Scheduler -> run worker job
RUN_URI="https://run.googleapis.com/v2/projects/${PROJECT_ID}/locations/${REGION}/jobs/${WORKER_JOB}:run"
gcloud --project "${PROJECT_ID}" projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SCHEDULER_SA}" \
  --role "roles/run.developer" >/dev/null
grant_service_account_user "${WORKER_SA}" "serviceAccount:${SCHEDULER_SA}"
gcloud --project "${PROJECT_ID}" scheduler jobs describe calendarsync-run-worker >/dev/null 2>&1 && \
  gcloud --project "${PROJECT_ID}" scheduler jobs delete calendarsync-run-worker --quiet
gcloud --project "${PROJECT_ID}" scheduler jobs create http calendarsync-run-worker \
  --location "${REGION}" \
  --schedule "* * * * *" \
  --uri "${RUN_URI}" \
  --http-method POST \
  --oauth-service-account-email "${SCHEDULER_SA}" \
  --oauth-token-scope "https://www.googleapis.com/auth/cloud-platform"

echo "Deployment completed."
echo "Web URL: ${WEB_URL}"
if [[ -n "${WEB_DOMAIN}" ]]; then
  echo "Custom web URL: https://${WEB_DOMAIN}"
fi
echo "API URL: ${API_URL}"
echo "Google OAuth redirect URL configured in API: ${API_REDIRECT_URL}"
echo "Remember to add this redirect URL to your Google OAuth client:"
echo "  ${API_REDIRECT_URL}"
if [[ -n "${WEB_DOMAIN}" ]]; then
  echo "Also add this authorized redirect URI for NextAuth:"
  echo "  https://${WEB_DOMAIN}/api/auth/callback/google"
  echo "And this authorized JavaScript origin:"
  echo "  https://${WEB_DOMAIN}"
fi
