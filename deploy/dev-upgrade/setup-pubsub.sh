#!/usr/bin/env bash
set -euo pipefail

# Create Pub/Sub topics/subscription with push endpoint + DLQ for dev-upgrade.
#
# Prereqs:
# - gcloud auth login
# - gcloud config set project cashflow-404ba
# - enable API: pubsub.googleapis.com
#
# Notes:
# - We use authenticated push to Cloud Run using OIDC.
# - Pub/Sub service agent must be able to mint tokens for the push service account.

PROJECT_ID="${PROJECT_ID:-cashflow-483906}"
REGION="${REGION:-asia-southeast1}"
SERVICE_NAME="${SERVICE_NAME:-cashflow-backend-dev-upgrade}"

TOPIC="${PUBSUB_TOPIC:-CashflowAccountingDevUpgrade}"
SUBSCRIPTION="${PUBSUB_SUBSCRIPTION:-CashflowAccountingDevUpgradeSub}"

DLQ_TOPIC="${DLQ_TOPIC:-CashflowAccountingDevUpgradeDLQ}"

PUSH_SA_NAME="${PUSH_SA_NAME:-pubsub-pusher-dev-upgrade}"
PUSH_SA_EMAIL="$PUSH_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com"

SERVICE_URL="$(gcloud run services describe "$SERVICE_NAME" --region "$REGION" --format='value(status.url)')"
if [[ -z "$SERVICE_URL" ]]; then
  echo "Could not determine Cloud Run URL for $SERVICE_NAME"
  exit 1
fi

PUSH_ENDPOINT="${SERVICE_URL%/}/pubsub"

echo "Using push endpoint: $PUSH_ENDPOINT"

# Ensure we're operating on the expected project.
gcloud config set project "$PROJECT_ID" 1>/dev/null

echo "Ensuring topics exist..."
gcloud pubsub topics create "$TOPIC" --quiet || true
gcloud pubsub topics create "$DLQ_TOPIC" --quiet || true

echo "Ensuring push service account exists..."
gcloud iam service-accounts create "$PUSH_SA_NAME" --display-name="Pub/Sub push to Cloud Run (dev-upgrade)" --quiet || true

echo "Grant Cloud Run invoker to push SA..."
gcloud run services add-iam-policy-binding "$SERVICE_NAME" \
  --region "$REGION" \
  --member "serviceAccount:$PUSH_SA_EMAIL" \
  --role "roles/run.invoker" \
  --quiet

PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')"
PUBSUB_SERVICE_AGENT="service-$PROJECT_NUMBER@gcp-sa-pubsub.iam.gserviceaccount.com"

echo "Allow Pub/Sub service agent to mint OIDC tokens for push SA..."
gcloud iam service-accounts add-iam-policy-binding "$PUSH_SA_EMAIL" \
  --member "serviceAccount:$PUBSUB_SERVICE_AGENT" \
  --role "roles/iam.serviceAccountTokenCreator" \
  --quiet

echo "Creating subscription with DLQ + retry policy..."
gcloud pubsub subscriptions create "$SUBSCRIPTION" \
  --topic "$TOPIC" \
  --push-endpoint="$PUSH_ENDPOINT" \
  --push-auth-service-account="$PUSH_SA_EMAIL" \
  --push-auth-token-audience="$SERVICE_URL" \
  --dead-letter-topic="$DLQ_TOPIC" \
  --max-delivery-attempts=10 \
  --min-retry-delay=10s \
  --max-retry-delay=600s \
  --ack-deadline=20 \
  --quiet || true

# If subscription already existed, ensure the push config is updated to the current service URL.
gcloud pubsub subscriptions update "$SUBSCRIPTION" \
  --push-endpoint="$PUSH_ENDPOINT" \
  --push-auth-service-account="$PUSH_SA_EMAIL" \
  --push-auth-token-audience="$SERVICE_URL" \
  --dead-letter-topic="$DLQ_TOPIC" \
  --max-delivery-attempts=10 \
  --min-retry-delay=10s \
  --max-retry-delay=600s \
  --ack-deadline=20 \
  --quiet

echo "Done."
echo "Main topic: $TOPIC"
echo "Subscription: $SUBSCRIPTION"
echo "DLQ topic: $DLQ_TOPIC"

