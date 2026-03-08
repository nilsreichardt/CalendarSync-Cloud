#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="open-calendar-sync"
REGION="${REGION:-europe-west1}"
REPO="calendarsync"
API_SERVICE="calendarsync-api"
WEB_SERVICE="calendarsync-web"
WORKER_JOB="calendarsync-worker"
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
RUNTIME_SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"
gcloud --project "${PROJECT_ID}" projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${RUNTIME_SA}" \
  --role "roles/secretmanager.secretAccessor" >/dev/null

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
  --allow-unauthenticated \
  --set-secrets "DATABASE_URL=calendarsync-neon-db:latest,GOOGLE_OAUTH_CLIENT_ID=calendarsync-google-client-id:latest,GOOGLE_OAUTH_CLIENT_SECRET=calendarsync-google-client-secret:latest,SCHEDULER_SHARED_SECRET=calendarsync-scheduler-secret:latest,OAUTH_STATE_SECRET_B64=calendarsync-oauth-state-secret-b64:latest,CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64=calendarsync-static-encryption-key-b64:latest" \
  --set-env-vars "GOOGLE_OAUTH_REDIRECT_URL=${API_REDIRECT_URL},DISPATCH_MIN_INTERVAL_SECONDS=30"

API_URL="$(gcloud --project "${PROJECT_ID}" run services describe "${API_SERVICE}" --region "${REGION}" --format='value(status.url)')"

gcloud --project "${PROJECT_ID}" run deploy "${WEB_SERVICE}" \
  --region "${REGION}" \
  --image "${WEB_IMAGE}" \
  --allow-unauthenticated \
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
  --set-secrets "DATABASE_URL=calendarsync-neon-db:latest,GOOGLE_OAUTH_CLIENT_ID=calendarsync-google-client-id:latest,GOOGLE_OAUTH_CLIENT_SECRET=calendarsync-google-client-secret:latest,CALENDARSYNC_STATIC_ENCRYPTION_KEY_B64=calendarsync-static-encryption-key-b64:latest"

# Scheduler -> dispatch endpoint
gcloud --project "${PROJECT_ID}" scheduler jobs describe calendarsync-dispatch >/dev/null 2>&1 && \
  gcloud --project "${PROJECT_ID}" scheduler jobs delete calendarsync-dispatch --quiet
gcloud --project "${PROJECT_ID}" scheduler jobs create http calendarsync-dispatch \
  --location "${REGION}" \
  --schedule "* * * * *" \
  --uri "${API_URL}/internal/scheduler/dispatch" \
  --http-method POST \
  --headers "X-Scheduler-Secret=${SCHEDULER_SECRET}"

# Scheduler -> run worker job
RUN_URI="https://run.googleapis.com/v2/projects/${PROJECT_ID}/locations/${REGION}/jobs/${WORKER_JOB}:run"
SCHEDULER_SA="calendarsync-scheduler@${PROJECT_ID}.iam.gserviceaccount.com"
gcloud --project "${PROJECT_ID}" iam service-accounts describe "${SCHEDULER_SA}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" iam service-accounts create calendarsync-scheduler --display-name "CalendarSync Scheduler"
gcloud --project "${PROJECT_ID}" projects add-iam-policy-binding "${PROJECT_ID}" \
  --member "serviceAccount:${SCHEDULER_SA}" \
  --role "roles/run.developer" >/dev/null
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
