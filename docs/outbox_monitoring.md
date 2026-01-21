## Outbox monitoring (Option C)

This project emits structured logs for outbox processing retries and DLQ (DB-side).

### Log fields to filter on

The worker paths emit a log entry with these fields on each failure:

- `field`: `OutboxProcessing`
- `business_id`
- `reference_type`
- `reference_id`
- `record_id`
- `processing_status`: `FAILED` or `DEAD`
- `process_attempts`

On success, `processing_status` is `SUCCEEDED`.

### Log-based metrics (Cloud Logging)

Create two counter metrics:

#### 1) Failed count

- **Metric name**: `outbox_failed_count`
- **Filter** (Cloud Run example):
  - `resource.type="cloud_run_revision"`
  - `jsonPayload.field="OutboxProcessing"`
  - `jsonPayload.processing_status="FAILED"`

#### 2) Dead / DLQ count

- **Metric name**: `outbox_dead_count`
- **Filter**:
  - `resource.type="cloud_run_revision"`
  - `jsonPayload.field="OutboxProcessing"`
  - `jsonPayload.processing_status="DEAD"`

If your logs arrive as `textPayload` (non-JSON), use a substring match instead:

- `textPayload:"field=OutboxProcessing" AND textPayload:"processing_status=DEAD"`

### Alert policies (Cloud Monitoring)

Create alerting policies based on the log-based metrics:

#### A) Failed spike

- **Condition**: `outbox_failed_count` rate or sum over 5 minutes above threshold
- **Suggested threshold**: start with `> 5` per 5 minutes (tune per environment)
- **Notification**: email/Slack/Pager

#### B) Any DLQ

- **Condition**: `outbox_dead_count` sum over 5 minutes `> 0`
- **Notification**: high priority (this means human action is required)

### Optional: oldest pending age

To alert on “stuck pending for too long”, you can add one of:

1) **Periodic SQL check** (recommended):
   - Run a scheduled query against `pub_sub_message_records` to compute the oldest row where:
     - `is_processed = 0`
     - `processing_status IN ('PENDING','FAILED','PROCESSING')`
   - Emit a log line with `outbox_oldest_pending_age_seconds`.
   - Create a gauge log-based metric and alert when above 15m/30m.

2) **App-level periodic log**:
   - Add a small periodic job in the service to log the same value.

### Operator runbook (quick)

- **See status in UI**: the document detail page shows the posting badge.
- **See status via API**: `getOutboxStatus(referenceType, referenceId)`
- **Reprocess (admin-only)**: `reprocessOutbox(referenceType, referenceId)`

