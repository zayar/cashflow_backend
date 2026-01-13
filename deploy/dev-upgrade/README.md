## Dev-upgrade backend deployment (Cloud Run + Pub/Sub push + DLQ)

### 1) Decide names
- **Backend GCP project**: `cashflow-483906` (where Cloud SQL + Memorystore live)
- **Frontend Firebase project**: `cashflow-404ba` (Hosting)
- **Cloud Run service**: `cashflow-backend-dev-upgrade`
- **Pub/Sub topic**: `CashflowAccountingDevUpgrade`
- **Subscription**: `CashflowAccountingDevUpgradeSub`
- **DLQ topic**: `CashflowAccountingDevUpgradeDLQ`

### 2) Set required secrets (recommended)
Create Secret Manager secret for DB password:

```bash
echo -n 'YOUR_DB_PASSWORD' | gcloud secrets create db-password-dev-upgrade --data-file=-
```

If it already exists:

```bash
echo -n 'YOUR_DB_PASSWORD' | gcloud secrets versions add db-password-dev-upgrade --data-file=-
```

### 3) Deploy Cloud Run
You must know your Cloud SQL connection name (format `PROJECT:REGION:INSTANCE`).

```bash
cd backend

export PROJECT_ID=cashflow-483906
export REGION=asia-southeast1
export SERVICE_NAME=cashflow-backend-dev-upgrade
export CLOUDSQL_CONNECTION_NAME="cashflow-483906:asia-southeast1:cashflow-mysql-dev-upgrade"
export REDIS_ADDRESS="10.85.205.115:6379"

# REQUIRED (because REDIS_ADDRESS is private):
# export VPC_CONNECTOR="YOUR_CONNECTOR_NAME"

./deploy/dev-upgrade/deploy-cloudrun.sh
```

This deploys with **authenticated access** (no unauthenticated traffic).

### 4) Wire Pub/Sub push + DLQ to `/pubsub`

```bash
export PROJECT_ID=cashflow-483906
export REGION=asia-southeast1
export SERVICE_NAME=cashflow-backend-dev-upgrade

./deploy/dev-upgrade/setup-pubsub.sh
```

### 5) Verify quickly
- Publish a test message (example payload must match your `config.PubSubMessage` schema):

```bash
gcloud pubsub topics publish CashflowAccountingDevUpgrade --message='{"id":1,"business_id":"<uuid>","reference_id":123,"reference_type":"Reconcile","action":"","old_obj":null,"new_obj":null,"transaction_date_time":"2026-01-10T00:00:00Z"}'
```

- Check Cloud Run logs: Pub/Sub should return **204** on success, **500** on processing errors (retries), and **204** for malformed payloads (drop/ack).

