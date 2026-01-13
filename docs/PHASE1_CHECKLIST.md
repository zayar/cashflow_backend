# Phase 1 Checklist — Stable Accounting & Architecture (200 merchants)

This checklist focuses on **accounting correctness**, **operational safety**, and **performance** for a multi-tenant system (~200 merchants).

---

## 1) Accounting correctness (non-negotiable)

- **Ledger immutability**
  - Ensure posted journals (`account_journals`, `account_transactions`) are **never updated/deleted**.
  - All changes must be done via **reversal journals** + (optional) “reposted” journal.
  - Implement linkage fields (now added in code):
    - `account_journals.is_reversal`
    - `account_journals.reverses_journal_id`
    - `account_journals.reversed_by_journal_id`
    - `account_journals.reversal_reason`
    - `account_journals.reversed_at`

- **Clear “document lifecycle” → “posting lifecycle”**
  - Draft documents must not post.
  - Confirmed/Approved documents post.
  - Void is a *posting action* (reversal), not a delete.
  - Keep a single source of truth: document status drives posting eligibility.

- **Period close / posting locks**
  - Keep the per-module lock dates (Sales/Purchase/Banking/Accountant).
  - Add a **hard posting gate**: worker rejects postings earlier than lock date (not only UI/API).
  - Add an explicit “Close Period” operation for accountants (audit trail + who/when).

- **Deterministic posting**
  - No random/non-deterministic behavior inside workflows.
  - Use stable rounding rules (e.g. 4 decimals everywhere, same rounding method).
  - Every workflow must end by setting `pub_sub_message_records.is_processed = true` only after all writes succeed.

- **Idempotency (already present)**
  - DB-backed idempotency key with unique constraint (good).
  - Ensure idempotency spans: **journal creation + inventory/balances updates + marking outbox processed** (single DB tx).

---

## 2) Outbox / messaging reliability (prevent silent data loss)

- **Transactional outbox (already present)**
  - Write outbox record in the same DB transaction.
  - Publish to Pub/Sub only after commit (dispatcher).

- **Outbox dispatcher must never “lose” messages**
  - **Recover stale PROCESSING** records if dispatcher crashes (reclaim lock after timeout).
  - Enforce **max publish attempts**; route poison messages to a terminal state:
    - `publish_status = DEAD` (DLQ equivalent) with error stored
  - Add a replay mechanism:
    - Admin action: move `DEAD → FAILED` (or new outbox row) after fixing root cause.

- **Consumer safety (already good)**
  - At-least-once delivery must be safe (idempotency).
  - Per-tenant ordering: prefer Pub/Sub ordering keys per `business_id` (Phase 2) or keep DB lock if you stay single region.

- **Observability**
  - Dashboards/alerts:
    - Outbox backlog by status (`PENDING/FAILED/PROCESSING/DEAD`)
    - Oldest pending age
    - Publish failures per minute
    - Worker failures per handler

---

## 3) Database schema & indexing (reporting + posting)

- **Outbox indexes**
  - Composite index for dispatcher scan: `(publish_status, next_attempt_at, id)`
  - Composite index for reconciliation scan: `(business_id, is_processed, id)`
  - Ensure `locked_at`/`locked_by` are indexed if you reclaim stale locks.

- **Idempotency indexes**
  - Unique: `(business_id, handler_name, message_id)` (already implemented)
  - Add index by `status` for operational inspection (already present).

- **Reporting performance**
  - Add/verify indexes for the most common filters:
    - `account_transactions (business_id, account_id, transaction_date_time)`
    - `account_transactions (journal_id)`
    - `account_journals (business_id, reference_type, reference_id)`
  - Avoid “full table scans” for trial balance / GL; pre-aggregation tables are OK but must be reconcilable.

---

## 4) Operations & security

- **Secrets**
  - No hardcoded credentials; use Secret Manager / Workload Identity.

- **Backups & recovery**
  - Cloud SQL automated backups + PITR.
  - Runbook for restoring + replaying outbox (reconciliation).

- **Tenant isolation**
  - Audit all queries for `business_id` scoping.
  - Add automated tests to prevent cross-tenant leaks.

---

## 5) Suggested “Phase 1 Exit Criteria”

- **0** stuck outbox rows in `PROCESSING` longer than `LockTimeout`.
- Any poison message ends in `DEAD`, visible in admin/reporting.
- Duplicate delivery test proven safe (idempotency) in a DB integration environment.
- Posted journals immutable; voids produce reversals, never deletes.

