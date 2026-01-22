#!/usr/bin/env bash
set -euo pipefail

# Deploy BOTH Cloud Run services for dev-upgrade:
# - API service (GraphQL)
# - Worker service (Pub/Sub push handler at /pubsub)
#
# This script simply runs deploy-cloudrun.sh twice with different SERVICE_NAME values.
#
# Usage:
#   export PROJECT_ID=cashflow-483906
#   export REGION=asia-southeast1
#   export REDIS_ADDRESS=10.x.x.x:6379
#   export VPC_CONNECTOR=your-connector-name   # if REDIS_ADDRESS is private
#   ./deploy/dev-upgrade/deploy-cloudrun-both.sh
#
# Optional overrides:
#   API_SERVICE_NAME=api-dev-upgrade-v2 WORKER_SERVICE_NAME=cashflow-backend-dev-upgrade ./deploy/dev-upgrade/deploy-cloudrun-both.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

API_SERVICE_NAME="${API_SERVICE_NAME:-api-dev-upgrade-v2}"
WORKER_SERVICE_NAME="${WORKER_SERVICE_NAME:-cashflow-backend-dev-upgrade}"

cd "$REPO_ROOT"

echo "Deploying API service: $API_SERVICE_NAME"
SERVICE_NAME="$API_SERVICE_NAME" SERVICE_ROLE="api" ./deploy/dev-upgrade/deploy-cloudrun.sh

echo
echo "Deploying WORKER service: $WORKER_SERVICE_NAME"
SERVICE_NAME="$WORKER_SERVICE_NAME" SERVICE_ROLE="worker" ./deploy/dev-upgrade/deploy-cloudrun.sh

echo
echo "Wiring Pub/Sub push subscription to WORKER /pubsub endpoint..."
SERVICE_NAME="$WORKER_SERVICE_NAME" ./deploy/dev-upgrade/setup-pubsub.sh

echo
echo "Done."

