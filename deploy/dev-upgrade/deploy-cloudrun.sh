#!/usr/bin/env bash
set -euo pipefail

# Deploy backend to Cloud Run for dev-upgrade.
#
# Prereqs:
# - gcloud auth login
# - gcloud config set project cashflow-404ba
# - enable APIs: run.googleapis.com, cloudbuild.googleapis.com, secretmanager.googleapis.com
#
# Required existing resources:
# - Cloud SQL instance (MySQL) + database
# - Memorystore (Redis) reachable from Cloud Run VPC connector (if using private IP)
# - Secret Manager secret for DB_PASSWORD (recommended)

PROJECT_ID="${PROJECT_ID:-cashflow-483906}"
REGION="${REGION:-asia-southeast1}"
SERVICE_NAME="${SERVICE_NAME:-cashflow-backend-dev-upgrade}"
SERVICE_ROLE="${SERVICE_ROLE:-}" # api|worker (recommended to set by deploy-cloudrun-both.sh)

# Cloud SQL connection name: PROJECT:REGION:INSTANCE
# Default instance name for this environment (override if needed).
CLOUDSQL_INSTANCE="${CLOUDSQL_INSTANCE:-cashflow-mysql-dev-upgrade}"

# Allow overriding the full connection name, otherwise derive it from PROJECT_ID/REGION/INSTANCE.
CLOUDSQL_CONNECTION_NAME="${CLOUDSQL_CONNECTION_NAME:-${PROJECT_ID}:${REGION}:${CLOUDSQL_INSTANCE}}"

if [[ -z "$CLOUDSQL_CONNECTION_NAME" ]]; then
  echo "Missing CLOUDSQL_CONNECTION_NAME (format PROJECT:REGION:INSTANCE)."
  echo "Example: ${PROJECT_ID}:${REGION}:${CLOUDSQL_INSTANCE}"
  exit 1
fi

# Optional VPC connector name if you need private Redis/SQL.
VPC_CONNECTOR="${VPC_CONNECTOR:-}"

# Pub/Sub vars
PUBSUB_TOPIC="${PUBSUB_TOPIC:-CashflowAccountingDevUpgrade}"
PUBSUB_SUBSCRIPTION="${PUBSUB_SUBSCRIPTION:-CashflowAccountingDevUpgradeSub}"

# Database vars
DB_USER="${DB_USER:-root}"
DB_NAME_2="${DB_NAME_2:-pitibooks}"
DB_PORT="${DB_PORT:-3306}"

# Redis vars
REDIS_ADDRESS="${REDIS_ADDRESS:-}"
if [[ -z "$REDIS_ADDRESS" ]]; then
  echo "Missing REDIS_ADDRESS (e.g. 10.0.0.5:6379)"
  exit 1
fi

# If Redis is on a private IP range, Cloud Run needs a Serverless VPC Connector.
if [[ "$REDIS_ADDRESS" =~ ^10\.|^192\.168\.|^172\.(1[6-9]|2[0-9]|3[0-1])\. ]]; then
  if [[ -z "$VPC_CONNECTOR" ]]; then
    echo "REDIS_ADDRESS looks private ($REDIS_ADDRESS) but VPC_CONNECTOR is not set."
    echo "Create a Serverless VPC Connector in $REGION and export VPC_CONNECTOR=<name>."
    exit 1
  fi
fi

# Other
TOKEN_HOUR_LIFESPAN="${TOKEN_HOUR_LIFESPAN:-24}"
GO_ENV="${GO_ENV:-dev-upgrade}"
STORAGE_PROVIDER="${STORAGE_PROVIDER:-gcs}"
GCS_BUCKET="${GCS_BUCKET:-}"
GCS_URL="${GCS_URL:-storage.googleapis.com}"
STORAGE_ACCESS_BASE_URL="${STORAGE_ACCESS_BASE_URL:-}"
GCS_SIGNER_EMAIL="${GCS_SIGNER_EMAIL:-}"
GCS_SIGNER_PRIVATE_KEY="${GCS_SIGNER_PRIVATE_KEY:-}"

if [[ -z "$SERVICE_ROLE" ]]; then
  # Backward compatible guess (safe default: worker).
  if [[ "$SERVICE_NAME" == "api-dev-upgrade-v2" ]]; then
    SERVICE_ROLE="api"
  else
    SERVICE_ROLE="worker"
  fi
fi

# Outbox controls (defaults depend on role)
# - API: do NOT process journals; do NOT run background outbox loops; do not accept Pub/Sub pushes.
# - Worker: process Pub/Sub pushes; (optionally) run outbox loops.
OUTBOX_DIRECT_PROCESSING="${OUTBOX_DIRECT_PROCESSING:-}"
ENABLE_PUBSUB_PUSH_ENDPOINT="${ENABLE_PUBSUB_PUSH_ENDPOINT:-}"
OUTBOX_RUN_DISPATCHER="${OUTBOX_RUN_DISPATCHER:-}"
OUTBOX_RUN_DIRECT_PROCESSOR="${OUTBOX_RUN_DIRECT_PROCESSOR:-}"

if [[ -z "$OUTBOX_DIRECT_PROCESSING" ]]; then
  if [[ "$SERVICE_ROLE" == "api" ]]; then
    OUTBOX_DIRECT_PROCESSING="false"
  else
    # Keep legacy dev-upgrade behavior (direct processing safety net) unless explicitly overridden.
    OUTBOX_DIRECT_PROCESSING="true"
  fi
fi
if [[ -z "$ENABLE_PUBSUB_PUSH_ENDPOINT" ]]; then
  if [[ "$SERVICE_ROLE" == "api" ]]; then
    ENABLE_PUBSUB_PUSH_ENDPOINT="false"
  else
    ENABLE_PUBSUB_PUSH_ENDPOINT="true"
  fi
fi
if [[ -z "$OUTBOX_RUN_DISPATCHER" ]]; then
  if [[ "$SERVICE_ROLE" == "api" ]]; then
    OUTBOX_RUN_DISPATCHER="false"
  else
    OUTBOX_RUN_DISPATCHER="true"
  fi
fi
if [[ -z "$OUTBOX_RUN_DIRECT_PROCESSOR" ]]; then
  if [[ "$SERVICE_ROLE" == "api" ]]; then
    OUTBOX_RUN_DIRECT_PROCESSOR="false"
  else
    OUTBOX_RUN_DIRECT_PROCESSOR="true"
  fi
fi

if [[ "$STORAGE_PROVIDER" == "gcs" && -z "$GCS_BUCKET" ]]; then
  echo "Missing GCS_BUCKET for STORAGE_PROVIDER=gcs"
  exit 1
fi

echo "Deploying $SERVICE_NAME to Cloud Run ($PROJECT_ID / $REGION)"

# Ensure we're deploying to the expected project.
gcloud config set project "$PROJECT_ID" 1>/dev/null

AUTH_FLAG="--no-allow-unauthenticated"
if [[ "$SERVICE_ROLE" == "api" ]]; then
  # API must be reachable by browser/Firebase rewrites.
  # Auth is enforced at the app layer (session middleware), so Cloud Run can be public.
  AUTH_FLAG="--allow-unauthenticated"
fi

SERVICE_JSON=""
SERVICE_EXISTS=false
if gcloud run services describe "$SERVICE_NAME" --region "$REGION" --format='value(metadata.name)' 1>/dev/null 2>&1; then
  SERVICE_EXISTS=true
  SERVICE_JSON="$(gcloud run services describe "$SERVICE_NAME" --region "$REGION" --format=json)"
fi

# CRITICAL: DB connection vars (DB_HOST, DB_PORT, DB_USER, DB_NAME_2) must be literals, not secrets.
# If they're secret-typed on an existing service, remove them first so deploy can set them as literals.
if [[ "$SERVICE_EXISTS" == "true" ]]; then
  DB_VARS_TO_FORCE=()
  python3 - <<'PY' <<<"${SERVICE_JSON}" | while IFS= read -r key; do
import json,sys
try:
  data=json.load(sys.stdin)
except Exception:
  sys.exit(0)
env=[]
try:
  env=data["spec"]["template"]["spec"]["containers"][0].get("env",[])
except Exception:
  env=[]
for e in env:
  name=e.get("name","")
  if name in ("DB_HOST","DB_PORT","DB_USER","DB_NAME_2") and e.get("valueFrom"):
    print(name)
PY
    [[ -n "$key" ]] && DB_VARS_TO_FORCE+=("$key")
  done
  
  if (( ${#DB_VARS_TO_FORCE[@]} > 0 )); then
    echo "Removing secret-typed DB vars before deploy: ${DB_VARS_TO_FORCE[*]}"
    DB_VARS_CSV="$(IFS=, ; echo "${DB_VARS_TO_FORCE[*]}")"
    gcloud run services update "$SERVICE_NAME" \
      --region "$REGION" \
      --remove-env-vars "$DB_VARS_CSV" \
      --quiet || true
  fi
fi

# If a variable already exists on the service as a Secret (valueFrom), Cloud Run does NOT allow
# overwriting it with a literal value. Instead of failing deploy, we skip setting that variable
# and keep the existing value/type.
should_set_literal_env() {
  local key="$1"
  if [[ -z "${SERVICE_JSON}" ]]; then
    return 0
  fi
  python3 - "$key" <<'PY' <<<"${SERVICE_JSON}" >/dev/null 2>&1
import json,sys
key=sys.argv[1]
try:
  data=json.load(sys.stdin)
except Exception:
  sys.exit(0)  # no service json -> allow set
env=[]
try:
  env=data["spec"]["template"]["spec"]["containers"][0].get("env",[])
except Exception:
  env=[]
for e in env:
  if e.get("name")==key:
    # secret typed if valueFrom is present
    if e.get("valueFrom"):
      sys.exit(1)
    sys.exit(0)
sys.exit(0)
PY
}

ENV_VARS=()
add_env_literal() {
  local key="$1"
  local val="$2"
  # CRITICAL: DB connection vars must ALWAYS be set as literals (never skip).
  # If DB_HOST/DB_PORT/DB_USER/DB_NAME_2 are missing, the service will crash on startup.
  if [[ "$key" == "DB_HOST" || "$key" == "DB_PORT" || "$key" == "DB_USER" || "$key" == "DB_NAME_2" ]]; then
    ENV_VARS+=("$key=$val")
  elif should_set_literal_env "$key"; then
    ENV_VARS+=("$key=$val")
  else
    echo "Skipping $key (already configured as secret on $SERVICE_NAME)"
  fi
}

DEPLOY_ARGS=(
  run deploy "$SERVICE_NAME"
  --region "$REGION"
  --source .
  "$AUTH_FLAG"
  --add-cloudsql-instances "$CLOUDSQL_CONNECTION_NAME"
)

add_env_literal "API_PORT_2" "8080"
add_env_literal "DB_USER" "$DB_USER"
add_env_literal "DB_PORT" "$DB_PORT"
add_env_literal "DB_HOST" "/cloudsql/$CLOUDSQL_CONNECTION_NAME"
add_env_literal "DB_NAME_2" "$DB_NAME_2"
add_env_literal "REDIS_ADDRESS" "$REDIS_ADDRESS"
add_env_literal "TOKEN_HOUR_LIFESPAN" "$TOKEN_HOUR_LIFESPAN"
add_env_literal "GO_ENV" "$GO_ENV"
add_env_literal "PUBSUB_PROJECT_ID" "$PROJECT_ID"
add_env_literal "PUBSUB_TOPIC" "$PUBSUB_TOPIC"
add_env_literal "PUBSUB_SUBSCRIPTION" "$PUBSUB_SUBSCRIPTION"
add_env_literal "OUTBOX_DIRECT_PROCESSING" "$OUTBOX_DIRECT_PROCESSING"
add_env_literal "ENABLE_PUBSUB_PUSH_ENDPOINT" "$ENABLE_PUBSUB_PUSH_ENDPOINT"
add_env_literal "OUTBOX_RUN_DISPATCHER" "$OUTBOX_RUN_DISPATCHER"
add_env_literal "OUTBOX_RUN_DIRECT_PROCESSOR" "$OUTBOX_RUN_DIRECT_PROCESSOR"
add_env_literal "STORAGE_PROVIDER" "$STORAGE_PROVIDER"
add_env_literal "GCS_BUCKET" "$GCS_BUCKET"
add_env_literal "GCS_URL" "$GCS_URL"

if [[ -n "$STORAGE_ACCESS_BASE_URL" ]]; then
  add_env_literal "STORAGE_ACCESS_BASE_URL" "$STORAGE_ACCESS_BASE_URL"
fi
if [[ -n "$GCS_SIGNER_EMAIL" ]]; then
  add_env_literal "GCS_SIGNER_EMAIL" "$GCS_SIGNER_EMAIL"
fi
if [[ -n "$GCS_SIGNER_PRIVATE_KEY" ]]; then
  add_env_literal "GCS_SIGNER_PRIVATE_KEY" "$GCS_SIGNER_PRIVATE_KEY"
fi

if [[ -n "$VPC_CONNECTOR" ]]; then
  # IMPORTANT:
  # Use private-ranges-only so Redis (10.x) goes through the VPC connector,
  # but Cloud SQL connector + Google APIs are still reachable without requiring Cloud NAT.
  DEPLOY_ARGS+=(--vpc-connector "$VPC_CONNECTOR" --vpc-egress private-ranges-only)
fi

# Recommended: use Secret Manager for DB password.
# Create once:
#   echo -n 'your-db-password' | gcloud secrets create db-password-dev-upgrade --data-file=-
# or update:
#   echo -n 'your-db-password' | gcloud secrets versions add db-password-dev-upgrade --data-file=-
DEPLOY_ARGS+=(--set-secrets "DB_PASSWORD=db-password-dev-upgrade:latest")

if (( ${#ENV_VARS[@]} > 0 )); then
  ENV_VARS_CSV="$(IFS=, ; echo "${ENV_VARS[*]}")"
  DEPLOY_ARGS+=(--set-env-vars "$ENV_VARS_CSV")
fi

gcloud "${DEPLOY_ARGS[@]}"

SERVICE_URL="$(gcloud run services describe "$SERVICE_NAME" --region "$REGION" --format='value(status.url)')"
echo "Deployed: $SERVICE_URL"
echo "Next: run ./deploy/dev-upgrade/setup-pubsub.sh (it will wire push delivery to $SERVICE_URL/pubsub)"

