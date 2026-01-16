package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
)

type AccountTransactionReportConnection struct {
	Edges    []*AccountTransactionReportEdge `json:"edges"`
	PageInfo *models.PageInfo                `json:"pageInfo"`
}

type AccountTransactionReportEdge struct {
	Cursor string                     `json:"cursor"`
	Node   *models.AccountTransaction `json:"node"`
}

func PaginateAccountTransactionReport(ctx context.Context, limit *int, after *string, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int, accountIds []int) (*AccountTransactionReportConnection, error) {
	businessID, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessID == "" {
		return nil, errors.New("business ID is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	decodedCursor, cursorId := models.DecodeCompositeCursor(after)
	edges := make([]*AccountTransactionReportEdge, *limit)
	count := 0
	hasNextPage := false

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessID)

	if branchID != nil && *branchID > 0 {
		dbCtx = dbCtx.Where("branch_id = ?", *branchID)
	}

	// temp
	dbCtx = dbCtx.Where("transaction_date_time BETWEEN ? AND ?", fromDate, toDate)
	// Hide reversal journals from the transactional report by default.
	dbCtx = dbCtx.Where("journal_id IN (SELECT id FROM account_journals WHERE business_id = ? AND is_reversal = 0)", businessID)
	if len(accountIds) > 0 {
		dbCtx.Where("account_id IN ?", accountIds)
	}

	// DB query
	var results []*models.AccountTransaction
	if decodedCursor == "" {
		err = dbCtx.Order("transaction_date_time DESC").Limit(*limit + 1).Find(&results).Error
	} else {
		err = dbCtx.Order("transaction_date_time DESC").Limit(*limit+1).
			Where("transaction_date_time < ? OR (transaction_date_time = ? AND id < ?)", decodedCursor, decodedCursor, cursorId).
			Find(&results).Error
	}
	if err != nil {
		return nil, err
	}

	for _, result := range results {
		// If there are any elements left after the current page
		// we indicate that in the response
		if count == *limit {
			hasNextPage = true
		}

		if count < *limit {
			edges[count] = &AccountTransactionReportEdge{
				Cursor: models.EncodeCompositeCursor(result.TransactionDateTime.String(), cursorId),
				Node:   result,
			}
			count++
		}
	}

	pageInfo := models.PageInfo{
		StartCursor: "",
		EndCursor:   "",
		HasNextPage: &hasNextPage,
	}
	if count > 0 {
		pageInfo.StartCursor = models.EncodeCompositeCursor(edges[0].Node.TransactionDateTime.String(), edges[0].Node.ID)
		pageInfo.EndCursor = models.EncodeCompositeCursor(edges[count-1].Node.TransactionDateTime.String(), edges[count-1].Node.ID)
	}

	connection := AccountTransactionReportConnection{
		Edges:    edges[:count],
		PageInfo: &pageInfo,
	}

	return &connection, nil
}

func GetAllAccountTransactionReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int, accountIds []int) ([]*models.AccountTransaction, error) {

	businessID, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessID == "" {
		return nil, errors.New("business ID is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessID)

	if branchID != nil && *branchID > 0 {
		dbCtx = dbCtx.Where("branch_id = ?", *branchID)
	}

	// temp
	dbCtx = dbCtx.Where("transaction_date_time BETWEEN ? AND ?", fromDate, toDate)
	// Hide reversal journals from the transactional report by default.
	dbCtx = dbCtx.Where("journal_id IN (SELECT id FROM account_journals WHERE business_id = ? AND is_reversal = 0)", businessID)
	if len(accountIds) > 0 {
		dbCtx.Where("account_id IN ?", accountIds)
	}

	// DB query
	var results []*models.AccountTransaction
	err = dbCtx.Order("transaction_date_time DESC").Find(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}
