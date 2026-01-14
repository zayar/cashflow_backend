package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

func main() {
	businessID := flag.String("business-id", "", "Optional: backfill only one business (uuid string). If empty, backfills all businesses.")
	from := flag.String("from", "", "Optional: start date (YYYY-MM-DD). Defaults to business migration date.")
	to := flag.String("to", "", "Optional: end date (YYYY-MM-DD). Defaults to today in business timezone.")
	branchID := flag.Int("branch-id", 0, "Branch id to backfill (default 0 for company-wide)")
	flag.Parse()

	ctx := context.Background()
	// Explicit DB connect (config no longer connects DB in init()).
	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized (config.GetDB returned nil)")
		os.Exit(1)
	}

	// Ensure schema is up-to-date (creates daily_summaries if missing).
	models.MigrateTable()

	// Many model hooks expect actor fields, even though we mostly execute raw SQL.
	ctx = context.WithValue(ctx, utils.ContextKeyUserId, 0)
	ctx = context.WithValue(ctx, utils.ContextKeyUserName, "BackfillDailySummary")

	var businesses []models.Business
	bizQuery := db.WithContext(ctx).Model(&models.Business{})
	if strings.TrimSpace(*businessID) != "" {
		bizQuery = bizQuery.Where("id = ?", strings.TrimSpace(*businessID))
	}
	if err := bizQuery.Find(&businesses).Error; err != nil {
		fmt.Fprintf(os.Stderr, "failed to list businesses: %v\n", err)
		os.Exit(1)
	}
	if len(businesses) == 0 {
		fmt.Fprintln(os.Stderr, "no businesses found to backfill")
		return
	}

	for _, b := range businesses {
		bid := b.ID.String()
		tz := "Asia/Yangon"
		if strings.TrimSpace(b.Timezone) != "" {
			tz = strings.TrimSpace(b.Timezone)
		}

		start := strings.TrimSpace(*from)
		if start == "" {
			// Default to business migration date (converted to local date).
			d, err := utils.ConvertToDate(b.MigrationDate, tz)
			if err != nil {
				fmt.Fprintf(os.Stderr, "business %s: failed to convert migration date: %v\n", bid, err)
				continue
			}
			start = d.Format("2006-01-02")
		}

		end := strings.TrimSpace(*to)
		if end == "" {
			d, err := utils.ConvertToDate(time.Now().UTC(), tz)
			if err != nil {
				fmt.Fprintf(os.Stderr, "business %s: failed to convert now(): %v\n", bid, err)
				continue
			}
			end = d.Format("2006-01-02")
		}

		fmt.Printf("Backfilling daily_summaries business=%s branch=%d currency=%d from=%s to=%s\n",
			bid, *branchID, b.BaseCurrencyId, start, end)

		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Upsert summaries from account_currency_daily_balances + account main types.
			if err := tx.Exec(`
				INSERT INTO daily_summaries (business_id, currency_id, branch_id, transaction_date, total_income, total_expense, created_at, updated_at)
				SELECT
					acb.business_id,
					acb.currency_id,
					acb.branch_id,
					acb.transaction_date,
					COALESCE(SUM(CASE WHEN a.main_type = 'Income' THEN -acb.balance ELSE 0 END), 0) AS total_income,
					COALESCE(SUM(CASE WHEN a.main_type = 'Expense' THEN  acb.balance ELSE 0 END), 0) AS total_expense,
					NOW(),
					NOW()
				FROM account_currency_daily_balances acb
				JOIN accounts a ON a.id = acb.account_id
				WHERE
					acb.business_id = ?
					AND acb.currency_id = ?
					AND acb.branch_id = ?
					AND acb.transaction_date BETWEEN ? AND ?
					AND a.main_type IN ('Income', 'Expense')
				GROUP BY
					acb.business_id, acb.currency_id, acb.branch_id, acb.transaction_date
				ON DUPLICATE KEY UPDATE
					total_income = VALUES(total_income),
					total_expense = VALUES(total_expense),
					updated_at = NOW()
			`, bid, b.BaseCurrencyId, *branchID, start, end).Error; err != nil {
				return err
			}

			// Delete stale rows (dates that no longer have any income/expense activity).
			return tx.Exec(`
				DELETE ds
				FROM daily_summaries ds
				LEFT JOIN (
					SELECT
						acb.business_id,
						acb.currency_id,
						acb.branch_id,
						acb.transaction_date
					FROM account_currency_daily_balances acb
					JOIN accounts a ON a.id = acb.account_id
					WHERE
						acb.business_id = ?
						AND acb.currency_id = ?
						AND acb.branch_id = ?
						AND acb.transaction_date BETWEEN ? AND ?
						AND a.main_type IN ('Income', 'Expense')
					GROUP BY
						acb.business_id, acb.currency_id, acb.branch_id, acb.transaction_date
				) agg
					ON agg.business_id = ds.business_id
					AND agg.currency_id = ds.currency_id
					AND agg.branch_id = ds.branch_id
					AND agg.transaction_date = ds.transaction_date
				WHERE
					ds.business_id = ?
					AND ds.currency_id = ?
					AND ds.branch_id = ?
					AND ds.transaction_date BETWEEN ? AND ?
					AND agg.transaction_date IS NULL
			`, bid, b.BaseCurrencyId, *branchID, start, end, bid, b.BaseCurrencyId, *branchID, start, end).Error
		}); err != nil {
			fmt.Fprintf(os.Stderr, "business %s backfill failed: %v\n", bid, err)
			continue
		}
	}

	fmt.Println("Backfill complete")
}
