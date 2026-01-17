# Inventory Truth Model

## 1) System map (current code before fix)
- **Products list – Stock in hand** → GraphQL resolvers `productResolver.StockInHand` / `productVariantResolver.StockInHand` (`graph/schema.resolvers.go`) → `models.GetStockInHand` → **`stock_summaries.current_qty`** aggregated across warehouses.
- **Inventory Summary report** → `reports.GetInventorySummaryReport` → SQL over **`stock_summary_daily_balances`** (aggregates of `stock_summaries`).
- **Stock Summary report** → `reports.GetStockSummaryReport` → SQL over **`stock_summary_daily_balances`** (range aggregation).
- **Warehouse report** → `reports.GetWarehouseInventoryReport` → SQL over **`stock_summary_daily_balances`**.
- **Inventory Valuation Summary report** → `reports.GetInventoryValuationSummaryReport` → SQL over **`stock_histories`** with `opening_stocks` fallback.
- **Inventory Valuation detail** → `models.GetInventoryValuation` → **`stock_histories`** (ledger-of-record).
- **Posting flows**:
  - Purchase/Bill receipts, Sales Invoice, Transfer Order, Inventory Adjustment (qty/value) use `stockCommands_*` helpers to mutate `stock_summaries`/daily balances; valuation/COGS and immutable ledger rows are posted via workflows (`workflow/*.go`) into **`stock_histories`**.
  - Backdated rebuild support exists in `workflow/inventoryRebuild.go` and is invoked from incoming stock processing.

## 2) Root causes of the mismatched numbers
- **Mixed sources**: Product page uses `stock_summaries`; valuation uses `stock_histories`; summary reports use `stock_summary_daily_balances`. When caches drift from ledger, screens disagree.
- **Warehouse scope mismatch**: Product page sums all warehouses; Warehouse/Inventory Summary can be filtered to a single warehouse, producing different totals (e.g., 115 vs 105).
- **Stale caches**: Backdated postings update `stock_histories` but caches (`stock_summaries`/daily balances) were not rebuilt deterministically, leaving divergent quantities.
- **Tie-breaking/ordering differences**: Caches depend on previous closing_qty ordering; ledger queries order by (stock_date, id). Stale cumulative fields lead to different on-hand snapshots.
- **Status filtering**: `stock_histories` already excludes reversals; caches could include residual committed/order quantities that no longer match ledger reality.

## 3) Truth model and canonical services (after fix)
- **Single source of truth**: `stock_histories` (append-only, non-reversed rows) are canonical for both quantity and valuation.
- **Canonical APIs** (in `models/inventoryTruthService.go`):
  - `InventorySnapshotByProductWarehouse(ctx, asOf, warehouseId, productId, productType, batchNumber)`
  - `InventorySnapshotByProduct(ctx, asOf, productId, productType)`
  - Shared aggregation sums `qty` and `qty*base_unit_value` ordered by `(stock_date, cumulative_sequence, id)`.
- **All screens now read the same ledger**:
  - Products StockInHand → `InventorySnapshotByProduct`.
  - Inventory Summary, Stock Summary, Warehouse Inventory → rewritten to aggregate `stock_histories`.
  - Inventory Valuation Summary/Detail already use `stock_histories`.
- **Negative stock guard**: `ensureNonNegativeForKeys` checks per (business, warehouse, product, batch) that cumulative qty never drops below zero after processing any stock workflow. Posting fails with a clear error.
- **Backdated handling**: Existing rebuild flow remains; per-workflow enforcement validates non-negative after recomputation.

## 4) Repair / backfill plan
1. **Dry-run on staging** with a copy of production data:
   - Run `go test ./...` with `INTEGRATION_TESTS=1` to exercise inventory flows.
2. **Full rebuild per business** (deterministic):
   - For each business/warehouse/product/batch, rerun `workflow.RebuildInventoryForItemWarehouseFromDate` starting from the earliest ledger date to regenerate closing balances and COGS.
   - After rebuild, run `ensureNonNegativeForKeys` to assert invariants.
3. **Cache refresh**:
   - Recompute `stock_summaries` and `stock_summary_daily_balances` from `stock_histories` (can be a one-off SQL job or small Go harness invoking the canonical snapshot to backfill).
4. **Operational command**:
   - Add/execute an admin CLI (e.g., `cmd/inventory-rebuild`) to rebuild a specific `(business, warehouse, product, batch, fromDate)` on demand.
5. **Monitoring**:
   - Enable `DEBUG_INVENTORY_VALUATION=1` and log min running qty after each posting until the fleet shows zero violations.

## 5) What to verify after backfill
- `SUM(onHand by warehouse) == onHand by product` for any as-of date.
- Inventory Valuation Summary quantity equals Stock Summary / Inventory Summary / Warehouse report for the same filters.
- Asset value equals `SUM(qty * base_unit_value)` over ledger as-of date.
- Backdated inserts trigger rebuild and keep the above invariants true.
