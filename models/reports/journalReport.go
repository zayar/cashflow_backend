package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
)

type JournalReportConnection struct {
	Edges    []*JournalReportEdge `json:"edges"`
	PageInfo *models.PageInfo     `json:"pageInfo"`
}

type JournalReportEdge struct {
	Cursor string                 `json:"cursor"`
	Node   *models.AccountJournal `json:"node"`
}

// func PaginateJournalReport(ctx context.Context, limit *int, after *string, fromDate time.Time, toDate time.Time, reportType string, branchID *int) (*JournalReportConnection, error) {
// 	businessID, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessID == "" {
// 		return nil, errors.New("business ID is required")
// 	}

// 	decodedCursor, _ := models.DecodeCursor(after)
// 	edges := make([]*JournalReportEdge, *limit)
// 	count := 0
// 	hasNextPage := false

// 	db := config.GetDB()
// 	dbCtx := db.WithContext(ctx).Preload("AccountTransactions").Where("business_id = ?", businessID)

// 	if branchID != nil && *branchID > 0 {
// 		dbCtx = dbCtx.Where("branch_id = ?", *branchID)
// 	}

// 	dbCtx = dbCtx.Where("transaction_date_time BETWEEN ? AND ?", fromDate, toDate)

// 	// DB query
// 	var results []*models.AccountJournal
// 	var err error
// 	if decodedCursor == "" {
// 		err = dbCtx.Order("transaction_date_time DESC").Limit(*limit + 1).Find(&results).Error
// 	} else {
// 		err = dbCtx.Order("transaction_date_time DESC").Limit(*limit+1).Where("transaction_date_time < ?", decodedCursor).Find(&results).Error
// 	}
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, result := range results {
// 		// If there are any elements left after the current page
// 		// we indicate that in the response
// 		if count == *limit {
// 			hasNextPage = true
// 		}

// 		if count < *limit {
// 			edges[count] = &JournalReportEdge{
// 				Cursor: models.EncodeCursor(result.TransactionDateTime.String()),
// 				Node:   result,
// 			}
// 			count++
// 		}
// 	}

// 	pageInfo := models.PageInfo{
// 		StartCursor: "",
// 		EndCursor:   "",
// 		HasNextPage: &hasNextPage,
// 	}
// 	if count > 0 {
// 		pageInfo.StartCursor = models.EncodeCursor(edges[0].Node.TransactionDateTime.String())
// 		pageInfo.EndCursor = models.EncodeCursor(edges[count-1].Node.TransactionDateTime.String())
// 	}

// 	connection := JournalReportConnection{
// 		Edges:    edges[:count],
// 		PageInfo: &pageInfo,
// 	}

// 	return &connection, nil
// }

func PaginateJournalReport(ctx context.Context, limit *int, after *string, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) (*JournalReportConnection, error) {
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

	// Decode the cursor to get the transaction_date_time and the secondary identifier
	decodedCursor, cursorID := models.DecodeCompositeCursor(after)
	edges := make([]*JournalReportEdge, *limit)
	count := 0
	hasNextPage := false

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Preload("AccountTransactions").Where("business_id = ?", businessID)

	if branchID != nil && *branchID > 0 {
		dbCtx = dbCtx.Where("branch_id = ?", *branchID)
	}

	dbCtx = dbCtx.Where("transaction_date_time BETWEEN ? AND ?", fromDate, toDate)

	// DB query
	var results []*models.AccountJournal
	// var err error
	if decodedCursor == "" {
		err = dbCtx.Order("transaction_date_time DESC, id DESC").Limit(*limit + 1).Find(&results).Error
	} else {
		err = dbCtx.Order("transaction_date_time DESC, id DESC").
			Limit(*limit+1).
			Where("transaction_date_time < ? OR (transaction_date_time = ? AND id < ?)", decodedCursor, decodedCursor, cursorID).
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
			edges[count] = &JournalReportEdge{
				Cursor: models.EncodeCompositeCursor(result.TransactionDateTime.String(), result.ID),
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

	connection := JournalReportConnection{
		Edges:    edges[:count],
		PageInfo: &pageInfo,
	}

	return &connection, nil
}

func GetAllJournalReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) ([]*models.AccountJournal, error) {
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
	dbCtx := db.WithContext(ctx).Preload("AccountTransactions").Where("business_id = ?", businessID)

	if branchID != nil && *branchID > 0 {
		dbCtx = dbCtx.Where("branch_id = ?", *branchID)
	}

	dbCtx = dbCtx.Where("transaction_date_time BETWEEN ? AND ?", fromDate, toDate)

	// DB query
	var results []*models.AccountJournal
	// var err error
	err = dbCtx.Order("transaction_date_time DESC, id DESC").Find(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}
