#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-open-calendar-sync}"
PROJECT_NUMBER="${PROJECT_NUMBER:-}"
POOL_ID="${POOL_ID:-github-actions}"
PROVIDER_ID="${PROVIDER_ID:-github}"
SERVICE_ACCOUNT_ID="${SERVICE_ACCOUNT_ID:-github-deployer}"
SERVICE_ACCOUNT_NAME="${SERVICE_ACCOUNT_NAME:-GitHub CD Deployer}"
GITHUB_OWNER="${GITHUB_OWNER:-nilsreichardt}"
GITHUB_REPO="${GITHUB_REPO:-CalendarSync-Cloud}"
BRANCH="${BRANCH:-main}"

usage() {
  cat <<EOF
Usage: deploy/setup_github_wif.sh

Bootstrap Workload Identity Federation for GitHub Actions deployments.

Optional environment variables:
  PROJECT_ID          GCP project id (default: open-calendar-sync)
  PROJECT_NUMBER      GCP project number (auto-discovered when omitted)
  POOL_ID             Workload Identity pool id (default: github-actions)
  PROVIDER_ID         Workload Identity provider id (default: github)
  SERVICE_ACCOUNT_ID  Service account id for GitHub deployments (default: github-deployer)
  SERVICE_ACCOUNT_NAME
                      Service account display name (default: GitHub CD Deployer)
  GITHUB_OWNER        GitHub org/user allowed to deploy (default: nilsreichardt)
  GITHUB_REPO         GitHub repository allowed to deploy (default: CalendarSync-Cloud)
  BRANCH              Git ref branch allowed to deploy (default: main)
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd gcloud

gcloud --project "${PROJECT_ID}" services enable \
  iam.googleapis.com \
  iamcredentials.googleapis.com \
  cloudresourcemanager.googleapis.com \
  sts.googleapis.com \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  artifactregistry.googleapis.com >/dev/null

if [[ -z "${PROJECT_NUMBER}" ]]; then
  PROJECT_NUMBER="$(gcloud --project "${PROJECT_ID}" projects describe "${PROJECT_ID}" --format='value(projectNumber)')"
fi

SA_EMAIL="${SERVICE_ACCOUNT_ID}@${PROJECT_ID}.iam.gserviceaccount.com"
POOL_NAME="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_ID}"
PROVIDER_NAME="${POOL_NAME}/providers/${PROVIDER_ID}"
REPO_SUBJECT="repo:${GITHUB_OWNER}/${GITHUB_REPO}:ref:refs/heads/${BRANCH}"

gcloud --project "${PROJECT_ID}" iam service-accounts describe "${SA_EMAIL}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" iam service-accounts create "${SERVICE_ACCOUNT_ID}" \
    --display-name "${SERVICE_ACCOUNT_NAME}"

gcloud --project "${PROJECT_ID}" iam workload-identity-pools describe "${POOL_ID}" \
  --location global >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" iam workload-identity-pools create "${POOL_ID}" \
    --location global \
    --display-name "GitHub Actions"

gcloud --project "${PROJECT_ID}" iam workload-identity-pools providers describe "${PROVIDER_ID}" \
  --location global \
  --workload-identity-pool "${POOL_ID}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" iam workload-identity-pools providers create-oidc "${PROVIDER_ID}" \
    --location global \
    --workload-identity-pool "${POOL_ID}" \
    --display-name "GitHub Actions" \
    --issuer-uri "https://token.actions.githubusercontent.com" \
    --attribute-mapping "google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository,attribute.repository_owner=assertion.repository_owner,attribute.ref=assertion.ref" \
    --attribute-condition "assertion.repository == '${GITHUB_OWNER}/${GITHUB_REPO}' && assertion.ref == 'refs/heads/${BRANCH}'"

for role in \
  roles/cloudbuild.builds.editor \
  roles/run.admin \
  roles/artifactregistry.admin \
  roles/iam.serviceAccountUser
do
  gcloud --project "${PROJECT_ID}" projects add-iam-policy-binding "${PROJECT_ID}" \
    --member "serviceAccount:${SA_EMAIL}" \
    --role "${role}" >/dev/null
done

gcloud iam service-accounts add-iam-policy-binding "${SA_EMAIL}" \
  --project "${PROJECT_ID}" \
  --role "roles/iam.workloadIdentityUser" \
  --member "principalSet://iam.googleapis.com/${POOL_NAME}/attribute.repository/${GITHUB_OWNER}/${GITHUB_REPO}" >/dev/null

cat <<EOF
Workload Identity Federation configured.

GitHub repository variables:
  GCP_PROJECT_ID=${PROJECT_ID}
  GCP_REGION=europe-west1
  GCP_WORKLOAD_IDENTITY_PROVIDER=${PROVIDER_NAME}
  GCP_SERVICE_ACCOUNT=${SA_EMAIL}

This setup allows deployments only from:
  repository: ${GITHUB_OWNER}/${GITHUB_REPO}
  branch: ${BRANCH}

Add the variables above in GitHub Actions settings before enabling the deploy workflow.
EOF
