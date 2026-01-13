package models

import (
	"fmt"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func (e *Expense) AfterCreate(tx *gorm.DB) (err error) {

	ctx := tx.Statement.Context
	description, err := describeTotalAmountCreated(ctx, "Expense", e.CurrencyId, e.TotalAmount)
	if err != nil {
		return err
	}

	if err := SaveHistoryCreate(tx, e.ID, e, description); err != nil {
		return err
	}

	// // for CreateBankingTransaction
	// var account Account
	// if err := tx.Model(&Account{}).Where("id = ?", e.AssetAccountId).First(&account).Error; err != nil {
	// 	return err
	// }
	// // for CreateBankingTransaction
	// if account.DetailType == AccountDetailTypeCash ||
	// 	account.DetailType == AccountDetailTypeBank {

	// 	input := BankingTransaction{
	// 		BranchId:          e.BranchId,
	// 		BusinessId:        e.BusinessId,
	// 		TransactionDate:   e.ExpenseDate,
	// 		Amount:            e.TotalAmount,
	// 		ReferenceNumber:   e.ReferenceNumber,
	// 		TransactionNumber: e.ExpenseNumber,
	// 		TransactionId:     e.ID,
	// 		TransactionType:   BankingTransactionTypeExpense,
	// 		Description:       e.Notes,
	// 		FromAccountId:     e.AssetAccountId,
	// 		ToAccountId:       e.ExpenseAccountId,
	// 		CurrencyId:        e.CurrencyId,
	// 		ExchangeRate:      e.ExchangeRate,
	// 		TaxAmount:         e.TaxAmount,
	// 		CustomerId:        e.CustomerId,
	// 		SupplierId:        e.SupplierId,
	// 	}

	// 	err = tx.Create(&input).Error
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

func (e *Expense) BeforeUpdate(tx *gorm.DB) (err error) {
	description := "Expense Updated."
	if tx.Statement.Changed("TotalAmount") {
		currency, err := GetResource[Currency](tx.Statement.Context, e.CurrencyId)
		if err != nil {
			return err
		}
		newAmount := tx.Statement.Dest.(map[string]interface{})["TotalAmount"].(decimal.Decimal)
		description += fmt.Sprintf("Total amount changed from %s.%v to %s.%v.", currency.Symbol, e.TotalAmount, currency.Symbol, newAmount)
	}
	if err := SaveHistoryUpdate(tx, e.ID, e, description); err != nil {
		return err
	}

	return nil
}

func (e *Expense) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, e.ID, e, "Deleted Expense"); err != nil {
		return err
	}

	return nil
}

func (s *SupplierCredit) AfterCreate(tx *gorm.DB) (err error) {

	if s.CurrentStatus == SupplierCreditStatusConfirmed {

		ctx := tx.Statement.Context
		// businessId, ok := utils.GetBusinessIdFromContext(tx.Statement.Context)
		// if !ok || businessId == "" {
		// 	return errors.New("business id is required")
		// }
		// ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, businessId)

		// lock stock summary
		// fieldValues, _ := s.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, s.BusinessId, s.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, s.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SupplierCreditAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, item := range s.Details {
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryReceivedQty(tx, s.BusinessId, s.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Neg(), s.SupplierCreditDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	description, err := describeTotalAmountCreated(tx.Statement.Context, "SupplierCredit", s.CurrencyId, s.SupplierCreditTotalAmount)
	if err != nil {
		return err
	}

	if err := SaveHistoryCreate(tx, s.ID, s, description); err != nil {
		return err
	}

	return nil
}

func (s *SupplierCredit) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, s.ID, s, "Updated SupplierCredit"); err != nil {
		return err
	}

	return nil
}

func (s *SupplierCredit) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, s.ID, s, "Deleted SupplierCredit"); err != nil {
		return err
	}

	return nil
}

func (s *SupplierPayment) AfterCreate(tx *gorm.DB) (err error) {
	description, err := describeTotalAmountCreated(tx.Statement.Context, "SupplierPayment", s.CurrencyId, s.Amount)
	if err != nil {
		return err
	}

	if err := SaveHistoryCreate(tx, s.ID, s, description); err != nil {
		return err
	}

	// // for CreateBankingTransaction
	// var account Account
	// if err := tx.Model(&Account{}).Where("id = ?", s.WithdrawAccountId).First(&account).Error; err != nil {
	// 	return err
	// }
	// // for CreateBankingTransaction
	// if account.DetailType == AccountDetailTypeCash ||
	// 	account.DetailType == AccountDetailTypeBank {

	// 	var detailItems []BankingTransactionDetail
	// 	for _, item := range s.PaidBills {
	// 		var bill Bill

	// 		if err := tx.First(&bill, item.BillId).Error; err != nil {
	// 			return err
	// 		}
	// 		detailItem := BankingTransactionDetail{
	// 			InvoiceNo:     bill.BillNumber,
	// 			DueAmount:     bill.BillTotalAmount.Sub(bill.BillTotalPaidAmount),
	// 			DueDate:       *bill.BillDueDate,
	// 			PaymentAmount: item.PaidAmount,
	// 		}
	// 		detailItems = append(detailItems, detailItem)
	// 	}
	// 	input := BankingTransaction{
	// 		BranchId:        s.BranchId,
	// 		BusinessId:      s.BusinessId,
	// 		TransactionDate: s.PaymentDate,
	// 		// Amount:            s.Amount.Add(s.BankCharges),
	// 		Amount:            s.Amount,
	// 		ReferenceNumber:   s.ReferenceNumber,
	// 		TransactionNumber: s.PaymentNumber,
	// 		TransactionType:   BankingTransactionTypeSupplierPayment,
	// 		Description:       s.Notes,
	// 		PaymentModeId:     s.PaymentModeId,
	// 		FromAccountId:     s.WithdrawAccountId,
	// 		CurrencyId:        s.CurrencyId,
	// 		ExchangeRate:      s.ExchangeRate,
	// 		BankCharges:       s.BankCharges,
	// 		SupplierId:        s.SupplierId,
	// 		Details:           detailItems,
	// 	}

	// 	err = tx.Create(&input).Error
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

func (s *SupplierPayment) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, s.ID, s, "Updated SupplierPayment"); err != nil {
		return err
	}

	return nil
}

func (s *SupplierPayment) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, s.ID, s, "Deleted SupplierPayment"); err != nil {
		return err
	}

	return nil
}

func (s *SalesOrder) AfterCreate(tx *gorm.DB) (err error) {
	// update committed qty if sale confirmed
	if s.CurrentStatus == SalesOrderStatusConfirmed {
		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := s.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, s.BusinessId, s.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, s.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SaleOrderAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, saleItem := range s.Details {
			if saleItem.ProductId > 0 {

				product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryCommittedQty(tx, s.BusinessId, s.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty, s.OrderDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	description, err := describeTotalAmountCreated(tx.Statement.Context, "SalesOrder", s.CurrencyId, s.OrderTotalAmount)
	if err != nil {
		return err
	}

	if err := SaveHistoryCreate(tx, s.ID, s, description); err != nil {
		return err
	}

	return nil
}

func (s *SalesOrder) BeforeUpdate(tx *gorm.DB) (err error) {

	// it need to test coz of errs
	// if err := SaveHistoryUpdate(tx, s.ID, s, "Updated SalesOrder"); err != nil {
	// 	return err
	// }

	return nil
}

func (s *SalesOrder) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, s.ID, s, "Deleted SalesOrder"); err != nil {
		return err
	}

	return nil
}

func (s *SalesInvoice) AfterCreate(tx *gorm.DB) (err error) {

	ctx := tx.Statement.Context
	if s.SalesOrderId > 0 && s.CurrentStatus == SalesInvoiceStatusConfirmed {
		if err := CloseSalesOrderStatus(tx, s); err != nil {
			tx.Rollback()
			return err
		}
	}

	if s.CurrentStatus == SalesInvoiceStatusConfirmed {
		// lock stock summary
		// fieldValues, _ := s.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, s.BusinessId, s.WarehouseId, fieldValues)
		// lock business
		err := utils.BusinessLock(ctx, s.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SaleInvoiceAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, saleItem := range s.Details {
			if saleItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}
				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummarySaleQty(tx, s.BusinessId, s.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty, s.InvoiceDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	description, err := describeTotalAmountCreated(tx.Statement.Context, "SalesInvoice", s.CurrencyId, s.InvoiceTotalAmount)
	if err != nil {
		return err
	}
	if err := SaveHistoryCreate(tx, s.ID, s, description); err != nil {
		return err
	}

	return nil
}

func (s *SalesInvoice) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, s.ID, s, "Updated SalesInvoice"); err != nil {
		return err
	}

	return nil
}

func (s *SalesInvoice) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, s.ID, s, "Deleted SalesInvoice"); err != nil {
		return err
	}

	return nil
}

func (c *CreditNote) AfterCreate(tx *gorm.DB) (err error) {

	if c.CurrentStatus == CreditNoteStatusConfirmed {

		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := c.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, c.BusinessId, c.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, c.BusinessId, "stockLock", "modelHoodsStockSummary.go", "CreditNoteAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, saleItem := range c.Details {
			if saleItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}
				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryReceivedQty(tx, c.BusinessId, c.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty, c.CreditNoteDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	description, err := describeTotalAmountCreated(tx.Statement.Context, "CreditNote", c.CurrencyId, c.CreditNoteTotalAmount)
	if err != nil {
		return err
	}
	if err := SaveHistoryCreate(tx, c.ID, c, description); err != nil {
		return err
	}

	return nil
}

func (c *CreditNote) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, c.ID, c, "Updated CreditNote"); err != nil {
		return err
	}

	return nil
}

func (c *CreditNote) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, c.ID, c, "Deleted CreditNote"); err != nil {
		return err
	}

	return nil
}

func (c *CustomerPayment) AfterCreate(tx *gorm.DB) (err error) {
	description, err := describeTotalAmountCreated(tx.Statement.Context, "CustomerPayment", c.CurrencyId, c.Amount)
	if err != nil {
		return err
	}
	if err := SaveHistoryCreate(tx, c.ID, c, description); err != nil {
		return err
	}

	// // for CreateBankingTransaction
	// var account Account
	// if err := tx.Model(&Account{}).Where("id = ?", c.DepositAccountId).First(&account).Error; err != nil {
	// 	return err
	// }
	// // for CreateBankingTransaction
	// if account.DetailType == AccountDetailTypeCash ||
	// 	account.DetailType == AccountDetailTypeBank {

	// 	var detailItems []BankingTransactionDetail
	// 	for _, item := range c.PaidInvoices {
	// 		var invoice SalesInvoice

	// 		if err := tx.First(&invoice, item.InvoiceId).Error; err != nil {
	// 			return err
	// 		}
	// 		detailItem := BankingTransactionDetail{
	// 			InvoiceNo:     invoice.InvoiceNumber,
	// 			DueAmount:     invoice.InvoiceTotalAmount.Sub(invoice.InvoiceTotalPaidAmount),
	// 			DueDate:       *invoice.InvoiceDueDate,
	// 			PaymentAmount: item.PaidAmount,
	// 		}
	// 		detailItems = append(detailItems, detailItem)
	// 	}

	// 	input := BankingTransaction{
	// 		BranchId:        c.BranchId,
	// 		BusinessId:      c.BusinessId,
	// 		TransactionDate: c.PaymentDate,
	// 		// Amount:            c.Amount.Sub(c.BankCharges),
	// 		Amount:            c.Amount,
	// 		ReferenceNumber:   c.ReferenceNumber,
	// 		TransactionNumber: c.PaymentNumber,
	// 		TransactionId:     c.ID,
	// 		TransactionType:   BankingTransactionTypeCustomerPayment,
	// 		Description:       c.Notes,
	// 		PaymentModeId:     c.PaymentModeId,
	// 		ToAccountId:       c.DepositAccountId,
	// 		CurrencyId:        c.CurrencyId,
	// 		ExchangeRate:      c.ExchangeRate,
	// 		BankCharges:       c.BankCharges,
	// 		CustomerId:        c.CustomerId,
	// 		Details:           detailItems,
	// 	}

	// 	err = tx.Create(&input).Error
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

func (c *CustomerPayment) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, c.ID, c, "Updated CustomerPayment"); err != nil {
		return err
	}

	return nil
}

func (c *CustomerPayment) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, c.ID, c, "Deleted CustomerPayment"); err != nil {
		return err
	}

	return nil
}

func (r *RecurringBill) AfterCreate(tx *gorm.DB) (err error) {
	description, err := describeTotalAmountCreated(tx.Statement.Context, "RecurringBill", r.CurrencyId, r.BillTotalAmount)
	if err != nil {
		return err
	}
	if err := SaveHistoryCreate(tx, r.ID, r, description); err != nil {
		return err
	}

	return nil
}

func (r *RecurringBill) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, r.ID, r, "Updated RecurringBill"); err != nil {
		return err
	}

	return nil
}

func (r *RecurringBill) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, r.ID, r, "Deleted RecurringBill"); err != nil {
		return err
	}

	return nil
}

func (r *BankingTransaction) AfterCreate(tx *gorm.DB) (err error) {
	description, err := describeTotalAmountCreated(tx.Statement.Context, "BankingTransaction", r.CurrencyId, r.Amount)
	if err != nil {
		return err
	}
	if err := SaveHistoryCreate(tx, r.ID, r, description); err != nil {
		return err
	}

	return nil
}

func (r *BankingTransaction) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, r.ID, r, "Updated BankingTransaction"); err != nil {
		return err
	}

	return nil
}

func (r *BankingTransaction) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, r.ID, r, "Deleted BankingTransaction"); err != nil {
		return err
	}

	return nil
}

func (p *ProductOption) AfterCreate(tx *gorm.DB) (err error) {
	return nil
}

func (p *ProductOption) BeforeUpdate(tx *gorm.DB) (err error) {

	return nil
}

func (p *ProductOption) AfterDelete(tx *gorm.DB) (err error) {
	return nil
}
func (r *RoleModule) AfterCreate(tx *gorm.DB) (err error) {
	return nil
}

func (r *RoleModule) BeforeUpdate(tx *gorm.DB) (err error) {

	return nil
}

func (r *RoleModule) AfterDelete(tx *gorm.DB) (err error) {

	return nil
}

func (c *Customer) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, c.ID, c, "Created Customer"); err != nil {
		return err
	}

	return nil
}

func (c *Customer) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, c.ID, c, "Updated Customer"); err != nil {
		return err
	}

	return nil
}

func (c *Customer) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, c.ID, c, "Deleted Customer"); err != nil {
		return err
	}

	return nil
}

func (j *JournalTransaction) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, j.ID, j, "Created JournalTransaction"); err != nil {
		return err
	}

	return nil
}

func (j *JournalTransaction) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, j.ID, j, "Updated JournalTransaction"); err != nil {
		return err
	}

	return nil
}

func (j *JournalTransaction) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, j.ID, j, "Deleted JournalTransaction"); err != nil {
		return err
	}

	return nil
}

func (p *ProductGroup) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, p.ID, p, "Created ProductGroup"); err != nil {
		return err
	}

	return nil
}

func (p *ProductGroup) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, p.ID, p, "Updated ProductGroup"); err != nil {
		return err
	}
	if err := p.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (p *ProductGroup) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, p.ID, p, "Deleted ProductGroup"); err != nil {
		return err
	}
	if err := p.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (c *Comment) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, c.ID, c, "Created Comment"); err != nil {
		return err
	}

	return nil
}

func (c *Comment) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, c.ID, c, "Updated Comment"); err != nil {
		return err
	}
	if err := c.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (c *Comment) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, c.ID, c, "Deleted Comment"); err != nil {
		return err
	}
	if err := c.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (c *CurrencyExchange) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, c.ID, c, "Created CurrencyExchange"); err != nil {
		return err
	}

	return nil
}

func (c *CurrencyExchange) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, c.ID, c, "Updated CurrencyExchange"); err != nil {
		return err
	}
	if err := c.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (c *CurrencyExchange) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, c.ID, c, "Deleted CurrencyExchange"); err != nil {
		return err
	}
	if err := c.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (j *Journal) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, j.ID, j, "Created Journal"); err != nil {
		return err
	}

	return nil
}

func (j *Journal) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, j.ID, j, "Updated Journal"); err != nil {
		return err
	}
	if err := j.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (j *Journal) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, j.ID, j, "Deleted Journal"); err != nil {
		return err
	}
	if err := j.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (m *Module) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, m.ID, m, "Created Module"); err != nil {
		return err
	}

	return nil
}

func (m *Module) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, m.ID, m, "Updated Module"); err != nil {
		return err
	}
	return nil
}

func (m *Module) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, m.ID, m, "Deleted Module"); err != nil {
		return err
	}
	return nil
}

func (p *Product) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, p.ID, p, "Created Product"); err != nil {
		return err
	}

	return nil
}

func (p *Product) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, p.ID, p, "Updated Product"); err != nil {
		return err
	}
	if err := p.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (p *Product) AfterSave(tx *gorm.DB) (err error) {
	ctx := tx.Statement.Context
	biz, _ := GetBusinessById(ctx, p.BusinessId)
	biz.ProcessProductIntegrationWorkflow(tx, p.ID)
	return nil
}

func (p *Product) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, p.ID, p, "Deleted Product"); err != nil {
		return err
	}
	if err := p.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (p *ProductVariant) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, p.ID, p, "Created ProductVariant"); err != nil {
		return err
	}

	return nil
}

func (p *ProductVariant) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, p.ID, p, "Updated ProductVariant"); err != nil {
		return err
	}
	if err := p.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (p *ProductVariant) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, p.ID, p, "Deleted ProductVariant"); err != nil {
		return err
	}
	if err := p.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (s *Supplier) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, s.ID, s, "Created Supplier"); err != nil {
		return err
	}

	return nil
}

func (s *Supplier) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, s.ID, s, "Updated Supplier"); err != nil {
		return err
	}
	if err := s.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (s *Supplier) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, s.ID, s, "Deleted Supplier"); err != nil {
		return err
	}
	if err := s.RemoveInstanceRedis(); err != nil {
		return err
	}

	return nil
}

func (t *TransactionNumberSeries) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, t.ID, t, "Created TransactionNumberSeries"); err != nil {
		return err
	}

	return nil
}

func (t *TransactionNumberSeries) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, t.ID, t, "Updated TransactionNumberSeries"); err != nil {
		return err
	}
	if err := t.RemoveInstanceRedis(); err != nil {
		return err
	}

	relatedBranchIds, err := t.getBranchIds(tx.Statement.Context)
	if err != nil {
		return err
	}
	for _, branchId := range relatedBranchIds {
		if err := config.RemoveRedisKey("tnsPrefixMap:" + fmt.Sprint(branchId)); err != nil {
			return err
		}
	}

	return nil
}

func (t *TransactionNumberSeries) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, t.ID, t, "Deleted TransactionNumberSeries"); err != nil {
		return err
	}
	if err := t.RemoveInstanceRedis(); err != nil {
		return err
	}

	relatedBranchIds, err := t.getBranchIds(tx.Statement.Context)
	if err != nil {
		return err
	}
	for _, branchId := range relatedBranchIds {
		if err := config.RemoveRedisKey("tnsPrefixMap:" + fmt.Sprint(branchId)); err != nil {
			return err
		}
	}

	return nil
}

func (a *Account) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, a.ID, a, "Created Account"); err != nil {
		return err
	}
	if err := a.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (a *Account) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, a.ID, a, "Updated Account"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(a); err != nil {
		return err
	}

	return nil
}

func (a *Account) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, a.ID, a, "Deleted Account"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(a); err != nil {
		return err
	}

	return nil
}

func (b *Branch) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, b.ID, b, "Created Branch"); err != nil {
		return err
	}
	if err := b.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (b *Branch) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, b.ID, b, "Updated Branch"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(b); err != nil {
		return err
	}

	if err := config.RemoveRedisKey("tnsPrefixMap:" + fmt.Sprint(b.ID)); err != nil {
		return err
	}
	return nil
}

func (b *Branch) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, b.ID, b, "Deleted Branch"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(b); err != nil {
		return err
	}

	if err := config.RemoveRedisKey("tsnPrefixMap:" + fmt.Sprint(b.ID)); err != nil {
		return err
	}
	return nil
}

func (c *Currency) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, c.ID, c, "Created Currency"); err != nil {
		return err
	}
	if err := c.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (c *Currency) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, c.ID, c, "Updated Currency"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(c); err != nil {
		return err
	}

	return nil
}

func (c *Currency) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, c.ID, c, "Deleted Currency"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(c); err != nil {
		return err
	}

	return nil
}

func (m *MoneyAccount) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, m.ID, m, "Created MoneyAccount"); err != nil {
		return err
	}
	if err := m.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (m *MoneyAccount) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, m.ID, m, "Updated MoneyAccount"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(m); err != nil {
		return err
	}

	return nil
}

func (m *MoneyAccount) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, m.ID, m, "Deleted MoneyAccount"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(m); err != nil {
		return err
	}

	return nil
}

func (p *ProductCategory) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, p.ID, p, "Created ProductCategory"); err != nil {
		return err
	}
	if err := p.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (p *ProductCategory) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, p.ID, p, "Updated ProductCategory"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(p); err != nil {
		return err
	}

	return nil
}

func (p *ProductCategory) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, p.ID, p, "Deleted ProductCategory"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(p); err != nil {
		return err
	}

	return nil
}

func (p *ProductModifier) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, p.ID, p, "Created ProductModifier"); err != nil {
		return err
	}
	if err := p.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (p *ProductModifier) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, p.ID, p, "Updated ProductModifier"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(p); err != nil {
		return err
	}

	return nil
}

func (p *ProductModifier) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, p.ID, p, "Deleted ProductModifier"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(p); err != nil {
		return err
	}

	return nil
}

func (p *ProductUnit) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, p.ID, p, "Created ProductUnit"); err != nil {
		return err
	}
	if err := p.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (p *ProductUnit) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, p.ID, p, "Updated ProductUnit"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(p); err != nil {
		return err
	}

	return nil
}

func (p *ProductUnit) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, p.ID, p, "Deleted ProductUnit"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(p); err != nil {
		return err
	}

	return nil
}

func (r *Role) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, r.ID, r, "Created Role"); err != nil {
		return err
	}
	if err := r.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (r *Role) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, r.ID, r, "Updated Role"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(r); err != nil {
		return err
	}

	return nil
}

func (r *Role) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, r.ID, r, "Deleted Role"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(r); err != nil {
		return err
	}

	return nil
}

func (r *Reason) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, r.ID, r, "Created Reason"); err != nil {
		return err
	}
	if err := r.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (r *Reason) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, r.ID, r, "Updated Reason"); err != nil {
		return err
	}
	if err := r.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (r *Reason) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, r.ID, r, "Deleted Reason"); err != nil {
		return err
	}
	if err := r.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (s *SalesPerson) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, s.ID, s, "Created SalesPerson"); err != nil {
		return err
	}
	if err := s.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (s *SalesPerson) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, s.ID, s, "Updated SalesPerson"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(s); err != nil {
		return err
	}

	return nil
}

func (s *SalesPerson) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, s.ID, s, "Deleted SalesPerson"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(s); err != nil {
		return err
	}

	return nil
}

func (t *Tax) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, t.ID, t, "Created Tax"); err != nil {
		return err
	}
	if err := t.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (t *Tax) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, t.ID, t, "Updated Tax"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(t); err != nil {
		return err
	}

	return nil
}

func (t *Tax) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, t.ID, t, "Deleted Tax"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(t); err != nil {
		return err
	}

	return nil
}

func (t *TaxGroup) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, t.ID, t, "Created TaxGroup"); err != nil {
		return err
	}
	if err := t.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (t *TaxGroup) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, t.ID, t, "Updated TaxGroup"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(t); err != nil {
		return err
	}

	return nil
}

func (t *TaxGroup) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, t.ID, t, "Deleted TaxGroup"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(t); err != nil {
		return err
	}

	return nil
}

func (w *Warehouse) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, w.ID, w, "Created Warehouse"); err != nil {
		return err
	}
	if err := w.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (w *Warehouse) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, w.ID, w, "Updated Warehouse"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(w); err != nil {
		return err
	}

	return nil
}

func (w *Warehouse) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, w.ID, w, "Deleted Warehouse"); err != nil {
		return err
	}
	if err := RemoveRedisBoth(w); err != nil {
		return err
	}

	return nil
}

func (dm *DeliveryMethod) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, dm.ID, dm, "Created DeliveryMethod"); err != nil {
		return err
	}
	if err := dm.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (dm *DeliveryMethod) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, dm.ID, dm, "Updated DeliveryMethod"); err != nil {
		return err
	}
	if err := dm.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (dm *DeliveryMethod) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, dm.ID, dm, "Deleted DeliveryMethod"); err != nil {
		return err
	}
	if err := dm.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (sp *ShipmentPreference) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, sp.ID, sp, "Created ShipmentPreference"); err != nil {
		return err
	}
	if err := sp.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (sp *ShipmentPreference) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, sp.ID, sp, "Updated ShipmentPreference"); err != nil {
		return err
	}
	if err := sp.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (sp *ShipmentPreference) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, sp.ID, sp, "Deleted ShipmentPreference"); err != nil {
		return err
	}
	if err := sp.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (pm *PaymentMode) AfterCreate(tx *gorm.DB) (err error) {
	if err := SaveHistoryCreate(tx, pm.ID, pm, "Created PaymentMode"); err != nil {
		return err
	}
	if err := pm.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (pm *PaymentMode) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, pm.ID, pm, "Updated PaymentMode"); err != nil {
		return err
	}
	if err := pm.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (pm *PaymentMode) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, pm.ID, pm, "Deleted PaymentMode"); err != nil {
		return err
	}
	if err := pm.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

// func getLastTransactions(tx *gorm.DB, currencyId int, branchId int, accountIds []int, transactionDateTime time.Time) ([]*AccountCurrencyDailyBalance, error) {

// 	var lastTransactions []*AccountCurrencyDailyBalance
// 	err := tx.Raw(`
// 		WITH LatestValues AS (
// 			SELECT
// 				*,
// 				ROW_NUMBER() OVER (PARTITION BY currency_id, branch_id, account_id ORDER BY transaction_date DESC) AS rn
// 			FROM
// 				account_currency_daily_balances
// 			WHERE currency_id = ? AND branch_id = ? AND account_id IN ? AND transaction_date < DATE(?)
// 		)
// 		SELECT
// 			*
// 		FROM
// 			LatestValues
// 		WHERE
// 			rn = 1
// 		`, currencyId, branchId, accountIds, transactionDateTime).Find(&lastTransactions).Error

// 	if err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	return lastTransactions, nil

// }

// func (b *BankingTransaction) AfterCreate(tx *gorm.DB) (err error) {
// 	// need to move after account_transactions

// 	accountIds := []int{b.FromAccountId, b.ToAccountId}
// 	accountDetailTypes := []string{string(AccountDetailTypeCash), string(AccountDetailTypeBank)}

// 	lastTransactions, err := getLastTransactions(tx, b.CurrencyId, b.BranchId, accountIds, b.TransactionDate)
// 	if err != nil {
// 		tx.Rollback()
// 		return err
// 	}

// 	for _, accountId := range accountIds {

// 		lastClosingBalance := decimal.NewFromInt(0)

// 		for _, transaction := range lastTransactions {
// 			if transaction.BranchId == b.BranchId && transaction.AccountId == accountId {
// 				lastClosingBalance = transaction.RunningBalance
// 				break
// 			}
// 		}

// 		err = tx.Exec(`
// 			WITH ranked_transactions AS (
// 				SELECT
// 					t.id,
// 					t.transaction_date_time,
// 					t.account_id,
// 					t.branch_id,
// 					t.base_debit,
// 					t.base_credit,
// 					? + SUM(t.base_debit - t.base_credit) OVER (
// 						PARTITION BY t.branch_id, t.account_id
// 						ORDER BY t.transaction_date_time, t.id
// 						ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
// 					) AS closing_balance,
// 					ROW_NUMBER() OVER (PARTITION BY t.branch_id, t.account_id ORDER BY t.transaction_date_time, t.id) AS rn
// 				FROM account_transactions t

// 				JOIN accounts acc On acc.id = t.account_id

// 				WHERE t.branch_id = ?
// 					AND account_id = ?
// 					AND transaction_date_time >= ?
// 					AND acc.detail_type IN (?)
// 			),
// 			ranked_banking AS (
// 				SELECT
// 					b.id,
// 					b.transaction_date,
// 					b.from_account_id,
// 					b.to_account_id,
// 					b.branch_id,
// 					ROW_NUMBER() OVER (ORDER BY b.transaction_date, b.id) AS rn
// 				FROM banking_transactions b

// 				JOIN accounts ad_from ON b.from_account_id = ad_from.id
// 				JOIN accounts ad_to ON b.to_account_id = ad_to.id

// 				WHERE b.branch_id = ?
// 					AND (b.from_account_id = ? OR b.to_account_id = ?)
// 					AND transaction_date >= ?
// 					AND (ad_from.detail_type IN (?) OR ad_to.detail_type IN (?))
// 			)

// 			UPDATE banking_transactions AS b

// 			JOIN ranked_banking rb ON b.id = rb.id
// 			LEFT JOIN ranked_transactions rt_from ON rb.rn = rt_from.rn
// 				AND rt_from.account_id = b.from_account_id
// 			LEFT JOIN ranked_transactions rt_to ON rb.rn = rt_to.rn
// 				AND rt_to.account_id = b.to_account_id

// 			SET
// 				b.from_account_closing_balance = COALESCE(rt_from.closing_balance, b.from_account_closing_balance),
// 				b.to_account_closing_balance = COALESCE(rt_to.closing_balance, b.to_account_closing_balance)

// 			WHERE b.branch_id = ?

// 			`, lastClosingBalance,
// 			b.BranchId, accountId, b.TransactionDate, accountDetailTypes,
// 			b.BranchId, accountId, accountId, b.TransactionDate, accountDetailTypes, accountDetailTypes,
// 			b.BranchId).Error

// 		if err != nil {
// 			tx.Rollback()
// 			return err
// 		}
// 	}
// 	return nil
// }

func (o *TransferOrder) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, o.ID, o, "Updated TransferOrder"); err != nil {
		return err
	}

	return nil
}

func (o *TransferOrder) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, o.ID, o, "Deleted TransferOrder"); err != nil {
		return err
	}

	return nil
}

func (o *InventoryAdjustment) BeforeUpdate(tx *gorm.DB) (err error) {
	if err := SaveHistoryUpdate(tx, o.ID, o, "Updated InventoryAdjustment"); err != nil {
		return err
	}

	return nil
}

func (o *InventoryAdjustment) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, o.ID, o, "Deleted InventoryAdjustment"); err != nil {
		return err
	}

	return nil
}

// func (r *Refund) AfterCreate(tx *gorm.DB) (err error) {
// 	var fromAccountId, toAccountId int
// 	if r.ReferenceType == RefundReferenceTypeCreditNote {
// 		fromAccountId = r.AccountId
// 		toAccountId = 0
// 	} else if r.ReferenceType == RefundReferenceTypeSupplierCredit {
// 		fromAccountId = r.AccountId
// 		toAccountId = 0
// 	}
// 	input := &BankingTransaction{
// 		BranchId:          input.BranchId,
// 		FromAccountId:     input.FromAccountId,
// 		ToAccountId:       input.ToAccountId,
// 		CustomerId:        input.CustomerId,
// 		SupplierId:        input.SupplierId,
// 		PaymentModeId:     input.PaymentModeId,
// 		TransactionDate:   input.TransactionDate,
// 		TransactionId:     input.TransactionId,
// 		TransactionNumber: input.TransactionNumber,
// 		TransactionType:   input.TransactionType,
// 		ExchangeRate:      input.ExchangeRate,
// 		CurrencyId:        input.CurrencyId,
// 		Amount:            input.Amount,
// 		TaxAmount:         input.TaxAmount,
// 		BankCharges:       input.BankCharges,
// 		ReferenceNumber:   input.ReferenceNumber,
// 		Description:       input.Description,
// 		Documents:         documents,
// 		Details:           detailItems,
// 		}

// 		_, err := CreateBankingTransaction(ctx, input)
// 		if err != nil {
// 			// handle error
// 		}
// 	return nil
// }
