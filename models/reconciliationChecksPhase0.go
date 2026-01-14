package models

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/sirupsen/logrus"
)

// RunPhase0ReconciliationChecks writes mismatch rows to reconciliation_reports.
// This is intended to be run on a schedule (nightly) or via an admin trigger.
func RunPhase0ReconciliationChecks(ctx context.Context, businessId string) (correlationId string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db := config.GetDB()
	if db == nil {
		return "", fmt.Errorf("db is nil")
	}
	logger := config.GetLogger()

	cid, ok := utils.GetCorrelationIdFromContext(ctx)
	if !ok || cid == "" {
		cid = uuid.NewString()
	}
	now := time.Now().UTC()

	// 1) Invoice totals vs journal presence + balance
	type invoiceRow struct{ ID int }
	var invoices []invoiceRow
	if err := db.WithContext(ctx).Raw(`
		SELECT si.id
		FROM sales_invoices si
		WHERE si.business_id = ? AND si.current_status IN ('Confirmed','Partial Paid','Paid','Write Off')
	`, businessId).Scan(&invoices).Error; err != nil {
		return cid, err
	}
	for _, inv := range invoices {
		var journalId *int
		_ = db.WithContext(ctx).Raw(`
			SELECT aj.id
			FROM account_journals aj
			WHERE aj.reference_type = 'IV'
			  AND aj.reference_id = ?
			  AND aj.is_reversal = 0
			  AND aj.reversed_by_journal_id IS NULL
			ORDER BY aj.id DESC
			LIMIT 1
		`, inv.ID).Scan(&journalId).Error
		if journalId == nil || *journalId == 0 {
			_ = db.WithContext(ctx).Create(&ReconciliationReport{
				BusinessId:    businessId,
				CheckType:     "INVOICE_JOURNAL",
				EntityType:    "SalesInvoice",
				EntityId:      inv.ID,
				Details:       "missing account_journal for invoice",
				CorrelationId: cid,
				CreatedAt:     now,
			}).Error
			continue
		}
		var imbalance int
		_ = db.WithContext(ctx).Raw(`
			SELECT
			  CASE
			    WHEN ROUND(SUM(at.base_debit), 4) = ROUND(SUM(at.base_credit), 4) THEN 0
			    ELSE 1
			  END AS imbalance
			FROM account_transactions at
			WHERE at.journal_id = ?
		`, *journalId).Scan(&imbalance).Error
		if imbalance == 1 {
			_ = db.WithContext(ctx).Create(&ReconciliationReport{
				BusinessId:    businessId,
				CheckType:     "INVOICE_JOURNAL",
				EntityType:    "AccountJournal",
				EntityId:      *journalId,
				Details:       "account_journal is not balanced (sum debits != sum credits)",
				CorrelationId: cid,
				CreatedAt:     now,
			}).Error
		}
	}

	// 2) Stock summary vs sum(stock_histories.qty)
	type stockMismatch struct {
		StockSummaryId int
		ExpectedQty    string
		ActualQty      string
	}
	var mismatches []stockMismatch
	if err := db.WithContext(ctx).Raw(`
		SELECT
			ss.id AS stock_summary_id,
			CAST(ss.current_qty AS CHAR) AS expected_qty,
			CAST(COALESCE(SUM(sh.qty), 0) AS CHAR) AS actual_qty
		FROM stock_summaries ss
		LEFT JOIN stock_histories sh
		  ON sh.business_id = ss.business_id
		 AND sh.warehouse_id = ss.warehouse_id
		 AND sh.product_id = ss.product_id
		 AND sh.product_type = ss.product_type
		 AND sh.batch_number = ss.batch_number
		WHERE ss.business_id = ?
		GROUP BY ss.id
		HAVING ROUND(ss.current_qty, 4) <> ROUND(COALESCE(SUM(sh.qty), 0), 4)
	`, businessId).Scan(&mismatches).Error; err != nil {
		return cid, err
	}
	for _, m := range mismatches {
		_ = db.WithContext(ctx).Create(&ReconciliationReport{
			BusinessId:    businessId,
			CheckType:     "STOCK_SUMMARY",
			EntityType:    "StockSummary",
			EntityId:      m.StockSummaryId,
			Details:       fmt.Sprintf("current_qty=%s != sum(stock_histories.qty)=%s", m.ExpectedQty, m.ActualQty),
			CorrelationId: cid,
			CreatedAt:     now,
		}).Error
	}

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"field":            "ReconciliationChecks",
			"business_id":      businessId,
			"correlation_id":   cid,
			"invoice_checked":  len(invoices),
			"stock_mismatches": len(mismatches),
		}).Info("phase0 reconciliation checks completed")
	}
	return cid, nil
}
