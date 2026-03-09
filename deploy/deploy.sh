#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-open-calendar-sync}"
REGION="${REGION:-europe-west1}"
REPO="${REPO:-calendarsync}"
BUILD_BACKEND="${BUILD_BACKEND:-cloudbuild}"
API_SERVICE="${API_SERVICE:-calendarsync-api}"
WORKER_JOB="${WORKER_JOB:-calendarsync-worker}"
API_SA="${API_SA:-calendarsync-api@${PROJECT_ID}.iam.gserviceaccount.com}"
WORKER_SA="${WORKER_SA:-calendarsync-worker@${PROJECT_ID}.iam.gserviceaccount.com}"
SCHEDULER_SA="${SCHEDULER_SA:-calendarsync-scheduler@${PROJECT_ID}.iam.gserviceaccount.com}"
KMS_CRYPTO_KEY="${KMS_CRYPTO_KEY:-}"
FRONTEND_URL="${FRONTEND_URL:-}"

usage() {
  cat <<'EOF'
Usage: deploy/deploy.sh [api] [worker]

Deploy one or more CalendarSync components by building a new image and updating
the existing Cloud Run service/job to that image.

Optional environment variables:
  PROJECT_ID   GCP project id (default: open-calendar-sync)
  REGION       Artifact Registry / Cloud Run region (default: europe-west1)
  REPO         Artifact Registry repository name (default: calendarsync)
  BUILD_BACKEND
               Build backend to use: cloudbuild or docker (default: cloudbuild)
  FRONTEND_URL Public frontend URL; if set, api deploy updates GOOGLE_OAUTH_REDIRECT_URL
  TAG          Image tag override (default: current git sha, or timestamp for dirty trees)

Examples:
  deploy/deploy.sh api
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
  COMPONENTS=(api)
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
  local component="$1"
  local image="$2"
  local cloudbuild_config=""
  local dockerfile=""
  local context=""

  case "${component}" in
    api)
      cloudbuild_config="deploy/cloudbuild.api.yaml"
      dockerfile="deploy/Dockerfile.api"
      context="."
      ;;
    worker)
      cloudbuild_config="deploy/cloudbuild.worker.yaml"
      dockerfile="deploy/Dockerfile.worker"
      context="."
      ;;
    *)
      echo "Unknown component for build: ${component}" >&2
      exit 1
      ;;
  esac

  case "${BUILD_BACKEND}" in
    cloudbuild)
      gcloud --project "${PROJECT_ID}" builds submit \
        --config "${cloudbuild_config}" \
        --substitutions "_IMAGE=${image}" .
      ;;
    docker)
      docker build -f "${dockerfile}" -t "${image}" "${context}"
      docker push "${image}"
      ;;
    *)
      echo "Unknown BUILD_BACKEND: ${BUILD_BACKEND}" >&2
      exit 1
      ;;
  esac
}

deploy_service() {
  local name="$1"
  local image="$2"
  shift 2

  gcloud --project "${PROJECT_ID}" run deploy "${name}" \
    --region "${REGION}" \
    --image "${image}" \
    "$@" \
    --quiet
}

deploy_job() {
  local name="$1"
  local image="$2"
  shift 2

  gcloud --project "${PROJECT_ID}" run jobs deploy "${name}" \
    --region "${REGION}" \
    --image "${image}" \
    "$@" \
    --quiet
}

require_cmd gcloud
if [[ "${BUILD_BACKEND}" == "docker" ]]; then
  require_cmd docker
fi

grant_run_invoker() {
  local service_name="$1"
  local member="$2"
  gcloud --project "${PROJECT_ID}" run services add-iam-policy-binding "${service_name}" \
    --region "${REGION}" \
    --member "${member}" \
    --role "roles/run.invoker" >/dev/null
}

gcloud --project "${PROJECT_ID}" artifacts repositories describe "${REPO}" \
  --location "${REGION}" >/dev/null 2>&1 || \
  gcloud --project "${PROJECT_ID}" artifacts repositories create "${REPO}" \
    --repository-format docker \
    --location "${REGION}" \
    --description "CalendarSync images"

if [[ "${BUILD_BACKEND}" == "docker" ]]; then
  gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet >/dev/null
fi

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
      build_image "api" "${IMAGE}"
      echo "Deploying service ${API_SERVICE}"
      EXTRA_ARGS=(--allow-unauthenticated --service-account "${API_SA}")
      if [[ -n "${KMS_CRYPTO_KEY}" ]]; then
        EXTRA_ARGS+=(--update-env-vars "KMS_CRYPTO_KEY=${KMS_CRYPTO_KEY}")
      fi
      if [[ -n "${FRONTEND_URL}" ]]; then
        EXTRA_ARGS+=(--update-env-vars "GOOGLE_OAUTH_REDIRECT_URL=${FRONTEND_URL%/}/oauth/google/callback")
      fi
      deploy_service "${API_SERVICE}" "${IMAGE}" "${EXTRA_ARGS[@]}"
      grant_run_invoker "${API_SERVICE}" "serviceAccount:${SCHEDULER_SA}"
      ;;
    worker)
      require_existing_service job "${WORKER_JOB}"
      IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${WORKER_JOB}:${TAG}"
      echo "Building ${component} image: ${IMAGE}"
      build_image "worker" "${IMAGE}"
      echo "Deploying job ${WORKER_JOB}"
      EXTRA_ARGS=(--service-account "${WORKER_SA}")
      if [[ -n "${KMS_CRYPTO_KEY}" ]]; then
        EXTRA_ARGS+=(--update-env-vars "KMS_CRYPTO_KEY=${KMS_CRYPTO_KEY}")
      fi
      deploy_job "${WORKER_JOB}" "${IMAGE}" "${EXTRA_ARGS[@]}"
      ;;
    *)
      if [[ "${component}" == "web" ]]; then
        echo "Frontend deployments moved to Vercel. Deploy the Next.js app from ./web there." >&2
      fi
      echo "Unknown component: ${component}" >&2
      usage >&2
      exit 1
      ;;
  esac
done

echo "Deployment completed."
