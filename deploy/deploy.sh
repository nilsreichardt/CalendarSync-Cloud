#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-open-calendar-sync}"
REGION="${REGION:-europe-west1}"
REPO="${REPO:-calendarsync}"
API_SERVICE="${API_SERVICE:-calendarsync-api}"
WEB_SERVICE="${WEB_SERVICE:-calendarsync-web}"
WORKER_JOB="${WORKER_JOB:-calendarsync-worker}"

usage() {
  cat <<'EOF'
Usage: deploy/deploy.sh [api] [web] [worker]

Deploy one or more CalendarSync components by building a new image in Cloud Build
and updating the existing Cloud Run service/job to that image.

Optional environment variables:
  PROJECT_ID   GCP project id (default: open-calendar-sync)
  REGION       Artifact Registry / Cloud Run region (default: europe-west1)
  REPO         Artifact Registry repository name (default: calendarsync)
  TAG          Image tag override (default: current git sha, or timestamp for dirty trees)

Examples:
  deploy/deploy.sh api web
  PROJECT_ID=open-calendar-sync REGION=europe-west1 deploy/deploy.sh worker

For first-time/bootstrap deployment that also creates secrets and scheduler jobs,
use deploy/e2e_deploy.sh instead.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$#" -eq 0 ]]; then
  COMPONENTS=(api web)
else
  COMPONENTS=("$@")
fi

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_existing_service() {
  local kind="$1"
  local name="$2"

  if [[ "$kind" == "service" ]]; then
    gcloud --project "${PROJECT_ID}" run services describe "${name}" \
      --region "${REGION}" >/dev/null
  else
    gcloud --project "${PROJECT_ID}" run jobs describe "${name}" \
      --region "${REGION}" >/dev/null
  fi
}

build_image() {
  local config="$1"
  local image="$2"

  gcloud --project "${PROJECT_ID}" builds submit \
    --config "${config}" \
    --substitutions "_IMAGE=${image}" .
}

deploy_service() {
  local name="$1"
  local image="$2"

  gcloud --project "${PROJECT_ID}" run deploy "${name}" \
    --region "${REGION}" \
    --image "${image}" \
    --quiet
}

deploy_job() {
  local name="$1"
  local image="$2"

  gcloud --project "${PROJECT_ID}" run jobs deploy "${name}" \
    --region "${REGION}" \
    --image "${image}" \
    --quiet
}

require_cmd gcloud

gcloud --project "${PROJECT_ID}" artifacts repositories describe "${REPO}" \
  --location "${REGION}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" artifacts repositories create "${REPO}" \
    --repository-format docker \
    --location "${REGION}" \
    --description "CalendarSync images"

TAG="${TAG:-}"
if [[ -z "${TAG}" ]]; then
  TAG="$(git rev-parse --short HEAD 2>/dev/null || true)"
  if [[ -z "${TAG}" || -n "$(git status --porcelain 2>/dev/null)" ]]; then
    TAG="$(date +%Y%m%d-%H%M%S)"
  fi
fi

for component in "${COMPONENTS[@]}"; do
  case "${component}" in
    api)
      require_existing_service service "${API_SERVICE}"
      IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${API_SERVICE}:${TAG}"
      echo "Building ${component} image: ${IMAGE}"
      build_image "deploy/cloudbuild.api.yaml" "${IMAGE}"
      echo "Deploying service ${API_SERVICE}"
      deploy_service "${API_SERVICE}" "${IMAGE}"
      ;;
    web)
      require_existing_service service "${WEB_SERVICE}"
      IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${WEB_SERVICE}:${TAG}"
      echo "Building ${component} image: ${IMAGE}"
      build_image "deploy/cloudbuild.web.yaml" "${IMAGE}"
      echo "Deploying service ${WEB_SERVICE}"
      deploy_service "${WEB_SERVICE}" "${IMAGE}"
      ;;
    worker)
      require_existing_service job "${WORKER_JOB}"
      IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${WORKER_JOB}:${TAG}"
      echo "Building ${component} image: ${IMAGE}"
      build_image "deploy/cloudbuild.worker.yaml" "${IMAGE}"
      echo "Deploying job ${WORKER_JOB}"
      deploy_job "${WORKER_JOB}" "${IMAGE}"
      ;;
    *)
      echo "Unknown component: ${component}" >&2
      usage >&2
      exit 1
      ;;
  esac
done

echo "Deployment completed."
