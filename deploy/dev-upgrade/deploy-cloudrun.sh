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

DEPLOY_ARGS=(
  run deploy "$SERVICE_NAME"
  --region "$REGION"
  --source .
  --no-allow-unauthenticated
  --add-cloudsql-instances "$CLOUDSQL_CONNECTION_NAME"
  --set-env-vars "API_PORT_2=8080"
  --set-env-vars "DB_USER=$DB_USER"
  --set-env-vars "DB_PORT=$DB_PORT"
  --set-env-vars "DB_HOST=/cloudsql/$CLOUDSQL_CONNECTION_NAME"
  --set-env-vars "DB_NAME_2=$DB_NAME_2"
  --set-env-vars "REDIS_ADDRESS=$REDIS_ADDRESS"
  --set-env-vars "TOKEN_HOUR_LIFESPAN=$TOKEN_HOUR_LIFESPAN"
  --set-env-vars "GO_ENV=$GO_ENV"
  --set-env-vars "PUBSUB_PROJECT_ID=$PROJECT_ID"
  --set-env-vars "PUBSUB_TOPIC=$PUBSUB_TOPIC"
  --set-env-vars "PUBSUB_SUBSCRIPTION=$PUBSUB_SUBSCRIPTION"
  --set-env-vars "OUTBOX_DIRECT_PROCESSING=$OUTBOX_DIRECT_PROCESSING"
  --set-env-vars "ENABLE_PUBSUB_PUSH_ENDPOINT=$ENABLE_PUBSUB_PUSH_ENDPOINT"
  --set-env-vars "OUTBOX_RUN_DISPATCHER=$OUTBOX_RUN_DISPATCHER"
  --set-env-vars "OUTBOX_RUN_DIRECT_PROCESSOR=$OUTBOX_RUN_DIRECT_PROCESSOR"
  --set-env-vars "STORAGE_PROVIDER=$STORAGE_PROVIDER"
  --set-env-vars "GCS_BUCKET=$GCS_BUCKET"
  --set-env-vars "GCS_URL=$GCS_URL"
)

if [[ -n "$STORAGE_ACCESS_BASE_URL" ]]; then
  DEPLOY_ARGS+=(--set-env-vars "STORAGE_ACCESS_BASE_URL=$STORAGE_ACCESS_BASE_URL")
fi
if [[ -n "$GCS_SIGNER_EMAIL" ]]; then
  DEPLOY_ARGS+=(--set-env-vars "GCS_SIGNER_EMAIL=$GCS_SIGNER_EMAIL")
fi
if [[ -n "$GCS_SIGNER_PRIVATE_KEY" ]]; then
  DEPLOY_ARGS+=(--set-env-vars "GCS_SIGNER_PRIVATE_KEY=$GCS_SIGNER_PRIVATE_KEY")
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

gcloud "${DEPLOY_ARGS[@]}"

SERVICE_URL="$(gcloud run services describe "$SERVICE_NAME" --region "$REGION" --format='value(status.url)')"
echo "Deployed: $SERVICE_URL"
echo "Next: run ./deploy/dev-upgrade/setup-pubsub.sh (it will wire push delivery to $SERVICE_URL/pubsub)"

