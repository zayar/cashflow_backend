-- Diagnostic SQL for Invoice IV-18 COGS = 0 issue
-- Business ID: cdf7f3f7-dce3-415c-8e4d-666ea3de98f0
-- Product: SSS
-- Invoice: IV-18

-- 1. Check invoice details
SELECT 
    si.id,
    si.invoice_number,
    si.invoice_date,
    si.current_status,
    si.branch_id,
    si.warehouse_id,
    sid.id as detail_id,
    sid.product_id,
    sid.qty,
    sid.cogs,
    sid.warehouse_id as detail_warehouse_id
FROM sales_invoices si
LEFT JOIN sales_invoice_details sid ON si.id = sid.sales_invoice_id
WHERE si.business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
    AND si.invoice_number LIKE '%IV-18%'
ORDER BY si.id DESC, sid.id;

-- 2. Check opening stock for product SSS
SELECT 
    sh.id,
    sh.stock_date,
    sh.warehouse_id,
    sh.product_id,
    sh.qty,
    sh.base_unit_value,
    sh.cumulative_incoming_qty,
    sh.cumulative_outgoing_qty,
    sh.closing_qty,
    sh.closing_asset_value,
    sh.reference_type,
    sh.reference_id,
    sh.is_outgoing,
    sh.is_reversal
FROM stock_histories sh
WHERE sh.business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
    AND sh.product_id IN (
        SELECT id FROM products 
        WHERE business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0' 
        AND name = 'SSS'
    )
ORDER BY sh.stock_date, sh.is_outgoing, sh.id;

-- 3. Check stock histories for IV-18 (including reversals)
SELECT 
    sh.id,
    sh.stock_date,
    sh.warehouse_id,
    sh.product_id,
    sh.qty,
    sh.base_unit_value,
    sh.cumulative_incoming_qty,
    sh.cumulative_outgoing_qty,
    sh.closing_qty,
    sh.closing_asset_value,
    sh.reference_type,
    sh.reference_id,
    sh.reference_detail_id,
    sh.is_outgoing,
    sh.is_reversal,
    sh.reversed_by_stock_history_id
FROM stock_histories sh
WHERE sh.business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
    AND (
        (sh.reference_type = 'INVOICE' AND sh.reference_id IN (
            SELECT id FROM sales_invoices 
            WHERE business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
            AND invoice_number LIKE '%IV-18%'
        ))
        OR sh.reference_type = 'REV-INVOICE'
    )
ORDER BY sh.stock_date, sh.is_outgoing, sh.id;

-- 4. Check journal entries for IV-18 and REV-IV-18
SELECT 
    aj.id as journal_id,
    aj.transaction_number,
    aj.reference_id,
    aj.reference_type,
    aj.transaction_date_time,
    at.id as transaction_id,
    at.account_id,
    a.account_code,
    a.account_name,
    at.base_debit,
    at.base_credit,
    at.is_inventory_valuation
FROM account_journals aj
JOIN account_transactions at ON aj.id = at.account_journal_id
JOIN accounts a ON at.account_id = a.id
WHERE aj.business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
    AND (
        (aj.reference_type = 'INVOICE' AND aj.reference_id IN (
            SELECT id FROM sales_invoices 
            WHERE business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
            AND invoice_number LIKE '%IV-18%'
        ))
        OR aj.transaction_number LIKE '%REV-IV-18%'
    )
ORDER BY aj.transaction_date_time, at.id;

-- 5. Check what incoming stock histories would be found by GetRemainingStockHistoriesByCumulativeQty
-- This simulates what calculateCogs would see when processing IV-18
SELECT 
    sh.id,
    sh.stock_date,
    sh.warehouse_id,
    sh.product_id,
    sh.qty,
    sh.base_unit_value,
    sh.cumulative_incoming_qty,
    sh.cumulative_outgoing_qty,
    sh.is_outgoing,
    sh.is_reversal,
    sh.reversed_by_stock_history_id
FROM stock_histories sh
WHERE sh.business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
    AND sh.warehouse_id = (
        SELECT warehouse_id FROM sales_invoices 
        WHERE business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0'
        AND invoice_number LIKE '%IV-18%'
        LIMIT 1
    )
    AND sh.product_id IN (
        SELECT id FROM products 
        WHERE business_id = 'cdf7f3f7-dce3-415c-8e4d-666ea3de98f0' 
        AND name = 'SSS'
    )
    AND sh.is_outgoing = false
    AND sh.cumulative_incoming_qty > 0  -- This is the filter used by GetRemainingStockHistoriesByCumulativeQty
    AND sh.is_reversal = 0
    AND sh.reversed_by_stock_history_id IS NULL
ORDER BY sh.stock_date, sh.is_outgoing, sh.id;
