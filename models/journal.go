package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// var mutex sync.Mutex

type Journal struct {
	ID                 int                  `gorm:"primary_key" json:"id"`
	BusinessId         string               `gorm:"index;not null" json:"business_id" binding:"required"`
	BranchId           int                  `gorm:"index" json:"branch_id"`
	JournalNumber      string               `gorm:"size:255;not null" json:"journal_number" binding:"required"`
	SequenceNo         decimal.Decimal      `gorm:"type:decimal(15);not null" json:"sequence_no"`
	ReferenceNumber    string               `gorm:"size:255" json:"reference_number"`
	JournalDate        time.Time            `gorm:"not null" json:"journal_date" binding:"required"`
	JournalNotes       string               `gorm:"type:text" json:"journal_notes" binding:"required"`
	CurrencyId         int                  `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate       decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	SupplierId         int                  `json:"supplierId"`
	CustomerId         int                  `json:"customerId"`
	JournalTotalAmount decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"journal_total_amount"`
	Transactions       []JournalTransaction `gorm:"foreignKey:JournalId" json:"transactions"`
	Documents          []*Document          `gorm:"polymorphic:Reference" json:"documents"`
	CreatedAt          time.Time            `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time            `gorm:"autoUpdateTime" json:"updated_at"`
}

type JournalTransaction struct {
	ID          int             `gorm:"primary_key" json:"id"`
	JournalId   int             `gorm:"index;not null" json:"journal_id" binding:"required"`
	AccountId   int             `gorm:"index;not null" json:"account_id" binding:"required"`
	BranchId    int             `json:"branch_id"`
	Description string          `gorm:"size:255" json:"description"`
	Debit       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"debit"`
	Credit      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"credit"`
}

type NewJournal struct {
	BranchId int `json:"branch_id"`
	// JournalNumber   string                  `json:"journal_number" binding:"required"`
	ReferenceNumber string                  `json:"reference_number"`
	JournalDate     time.Time               `json:"journal_date" binding:"required"`
	JournalNotes    string                  `json:"journal_notes"`
	CurrencyId      int                     `json:"currency_id" binding:"required"`
	ExchangeRate    decimal.Decimal         `json:"exchange_rate"`
	SupplierId      int                     `json:"supplier_id"`
	CustomerId      int                     `json:"customer_id"`
	Transactions    []NewJournalTransaction `json:"transactions"`
	Documents       []*NewDocument          `json:"documents"`
}

type NewJournalTransaction struct {
	AccountId   int             `json:"account_id" binding:"required"`
	BranchId    int             `json:"branch_id"`
	Description string          `json:"description"`
	Debit       decimal.Decimal `json:"debit"`
	Credit      decimal.Decimal `json:"credit"`
}

type JournalsConnection struct {
	Edges    []*JournalsEdge `json:"edges"`
	PageInfo *PageInfo       `json:"pageInfo"`
}

type JournalsEdge struct {
	Cursor string   `json:"cursor"`
	Node   *Journal `json:"node"`
}

func (j Journal) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, j.JournalDate, j.BusinessId, AccountantTransactionLock)
}

// GetID method for Journal reference Data
func (j *Journal) GetId() int {
	return j.ID
}

func (jt JournalTransaction) GetId() int {
	return jt.ID
}

func (jt JournalTransaction) fillable() map[string]interface{} {
	return map[string]interface{}{
		"AccountId":   jt.AccountId,
		"BranchId":    jt.BranchId,
		"Description": jt.Description,
		"Debit":       jt.Debit,
		"Credit":      jt.Credit,
	}
}

func upsertJournalTransaction(ctx context.Context, tx *gorm.DB, input []JournalTransaction, journalId int) error {
	return ReplaceAssociation(ctx, tx, input, "journal_id = ?", journalId)
}

// validate input for both create & update. (id = 0 for create)

func (input *NewJournal) validate(ctx context.Context, businessId string, _ int) error {
	// journal number
	// if err := utils.ValidateUnique[Journal](ctx, businessId, "name", input.Name, id); err != nil {
	// 	return err
	// }

	// branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	// currencyId
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}

	// exists supplier
	if input.SupplierId > 0 {
		if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
			return errors.New("supplier not found")
		}
	}

	// exists customer
	if input.CustomerId > 0 {
		if err := utils.ValidateResourceId[Customer](ctx, businessId, input.CustomerId); err != nil {
			return errors.New("customer not found")
		}
	}

	// validate transactionLock
	if err := validateTransactionLock(ctx, input.JournalDate, businessId, AccountantTransactionLock); err != nil {
		return err
	}
	return nil
}

func receiveJournalTransactions(input *NewJournal, journalId int) ([]JournalTransaction, decimal.Decimal, error) {
	transactions := make([]JournalTransaction, 0)
	totalAmount := decimal.NewFromInt(0)
	for _, t := range input.Transactions {
		if t.Debit.IsZero() && t.Credit.IsZero() {
			return transactions, totalAmount, errors.New("either debit or credit must have value")
		}
		totalAmount = totalAmount.Add(t.Debit)
		transactions = append(transactions, JournalTransaction{
			JournalId:   journalId,
			AccountId:   t.AccountId,
			BranchId:    t.BranchId,
			Description: t.Description,
			Debit:       t.Debit,
			Credit:      t.Credit,
		})
	}
	return transactions, totalAmount, nil
}

// func upsertJournalTr

// func GetJournalSequence(ctx context.Context, businessId string) (int64, error) {
// 	// lock
// 	fmt.Println("waiting: " + time.Now().Format("15:04:05"))
// 	mutex.Lock()
// 	fmt.Println("LOCKED: " + time.Now().Format("15:04:05"))
// 	defer mutex.Unlock()
// 	cacheKey := businessId + "-journal_seq"
// 	var seqNo int64
// 	var err error
// 	db := config.GetDB()

// 	for {
// 		seqNo, err = config.GetRedisCounter(ctx, cacheKey)
// 		if err != nil {
// 			return 0, err
// 		}
// 		// if not found in redis, get from db
// 		if seqNo == 1 {
// 			// get max seq no from db
// 			var dbSeq *int64
// 			if err := db.WithContext(ctx).Model(&Journal{}).Select("max(sequence_no)").
// 				Where("business_id = ?", businessId).
// 				Scan(&dbSeq).Error; err != nil {
// 				return 0, err
// 			}
// 			// in case db has no journal records
// 			if dbSeq == nil {
// 				seqNo = 0
// 			} else {
// 				seqNo = *dbSeq
// 			}
// 			// set redis
// 			seqNo++
// 			if err := config.SetRedisObject(cacheKey, &seqNo, 0); err != nil {
// 				return 0, err
// 			}
// 		}
// 		// check if sequence number exists in db
// 		err = utils.ValidateUnique[Journal](ctx, businessId, "sequence_no", seqNo, 0)
// 		if err == nil {
// 			break
// 		}
// 	}
// 	time.Sleep(time.Second * 5)
// 	fmt.Printf("FINISH seq no:%v\n", seqNo)
// 	// unlock
// 	return seqNo, nil
// }

func CreateJournal(ctx context.Context, input *NewJournal) (*Journal, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}
	transactions, totalAmount, err := receiveJournalTransactions(input, 0)
	if err != nil {
		return nil, err
	}
	documents, err := mapNewDocuments(input.Documents, "journals", 0)
	if err != nil {
		return nil, err
	}

	journal := Journal{
		BusinessId: businessId,
		BranchId:   input.BranchId,
		// JournalNumber:      input.JournalNumber,
		ReferenceNumber:    input.ReferenceNumber,
		JournalDate:        input.JournalDate,
		JournalNotes:       input.JournalNotes,
		CurrencyId:         input.CurrencyId,
		ExchangeRate:       input.ExchangeRate,
		SupplierId:         input.SupplierId,
		CustomerId:         input.CustomerId,
		JournalTotalAmount: totalAmount,

		Transactions: transactions,
		Documents:    documents,
	}
	seqNo, err := utils.GetSequence[Journal](ctx, businessId)
	// seqNo, err := utils.GetSequence[Journal](ctx, businessId)
	if err != nil {
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Manual Journal")
	if err != nil {
		return nil, err
	}
	journal.SequenceNo = decimal.NewFromInt(seqNo)
	journal.JournalNumber = prefix + fmt.Sprint(seqNo)

	db := config.GetDB()
	// db action
	tx := db.Begin()
	err = tx.WithContext(ctx).Create(&journal).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = PublishToAccounting(ctx, tx, businessId, journal.JournalDate, journal.ID, AccountReferenceTypeJournal, journal, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &journal, nil
}

func UpdateJournal(ctx context.Context, id int, input *NewJournal) (*Journal, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	transactions, totalAmount, err := receiveJournalTransactions(input, id)
	if err != nil {
		return nil, err
	}

	journal, err := utils.FetchModelForChange[Journal](ctx, businessId, id, "Transactions")
	if err != nil {
		return nil, err
	}
	oldJournal := *journal

	db := config.GetDB()
	// db action
	tx := db.Begin()
	err = tx.WithContext(ctx).Model(&journal).Updates(map[string]interface{}{
		"BranchId":           input.BranchId,
		"ReferenceNumber":    input.ReferenceNumber,
		"JournalDate":        input.JournalDate,
		"JournalNotes":       input.JournalNotes,
		"CurrencyId":         input.CurrencyId,
		"ExchangeRate":       input.ExchangeRate,
		"SupplierId":         input.SupplierId,
		"CustomerId":         input.CustomerId,
		"JournalTotalAmount": totalAmount,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := upsertJournalTransaction(ctx, tx, transactions, id); err != nil {
		tx.Rollback()
		return nil, err
	}

	if _, err := upsertDocuments(ctx, tx, input.Documents, "journals", id); err != nil {
		tx.Rollback()
		return nil, err
	}

	journal.Transactions = transactions
	err = PublishToAccounting(ctx, tx, businessId, journal.JournalDate, journal.ID, AccountReferenceTypeJournal, journal, oldJournal, PubSubMessageActionUpdate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return journal, nil
}

func DeleteJournal(ctx context.Context, id int) (*Journal, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	journal, err := utils.FetchModelForChange[Journal](ctx, businessId, id, "Transactions", "Documents")
	if err != nil {
		return nil, err
	}

	// db action
	tx := db.Begin()
	// delete associated transactions first
	if err := tx.WithContext(ctx).Model(&journal).Association("Transactions").
		Unscoped().Clear(); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := deleteDocuments(ctx, tx, journal.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.WithContext(ctx).Delete(&journal).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	err = PublishToAccounting(ctx, tx, businessId, journal.JournalDate, journal.ID, AccountReferenceTypeJournal, nil, journal, PubSubMessageActionDelete)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return journal, nil
}

func GetJournal(ctx context.Context, id int) (*Journal, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return utils.FetchModel[Journal](ctx, businessId, id, "Transactions")
}

func PaginateJournals(ctx context.Context, limit *int, after *string, journalNumber *string, fromDate *MyDateString, toDate *MyDateString, branchId *int, referenceNumber *string) (*JournalsConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	decodedCursor, _ := DecodeCursor(after)
	edges := make([]*JournalsEdge, *limit)
	count := 0
	hasNextPage := false

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Preload("Transactions").Where("business_id = ?", businessId)
	if journalNumber != nil && *journalNumber != "" {
		dbCtx.Where("journal_number LIKE ?", "%"+*journalNumber+"%")
	}
	if fromDate != nil && toDate != nil {
		dbCtx = dbCtx.Where("journal_date BETWEEN ? AND ?", fromDate, toDate)
	}
	if branchId != nil && *branchId > 0 {
		dbCtx.Where("branch_id = ?", branchId)
	}
	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}
	// if notes != nil && *notes != "" {
	// 	dbCtx.Where("journal_notes LIKE ?", "%"+*notes+"%")
	// }
	// db query
	var results []*Journal
	// err := dbCtx.Find(&results).Error
	if decodedCursor == "" {
		err = dbCtx.Order("created_at DESC").Limit(*limit + 1).Find(&results).Error
	} else {
		err = dbCtx.Order("created_at DESC").Limit(*limit+1).Where("created_at < ?", decodedCursor).Find(&results).Error
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
			edges[count] = &JournalsEdge{
				Cursor: EncodeCursor(result.CreatedAt.String()),
				Node:   result,
			}
			count++
		}
	}

	pageInfo := PageInfo{
		StartCursor: "",
		EndCursor:   "",
		HasNextPage: &hasNextPage,
	}
	if count > 0 {
		pageInfo.StartCursor = EncodeCursor(edges[0].Node.CreatedAt.String())
		pageInfo.EndCursor = EncodeCursor(edges[count-1].Node.CreatedAt.String())
	}

	connection := JournalsConnection{
		Edges:    edges[:count],
		PageInfo: &pageInfo,
	}

	return &connection, nil
}
