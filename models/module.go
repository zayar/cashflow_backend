package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type Module struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"index;not null" json:"business_id" binding:"required"`
	Name       string    `gorm:"index;size:100;not null" json:"name" binding:"required"`
	Actions    string    `gorm:"not null" json:"action" binding:"required"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewModule struct {
	Name       string `json:"name" binding:"required"`
	BusinessId string
	Actions    string `json:"action" binding:"required"`
}

/*
cache
	ModuleList:$businessId
*/

// get ids of roles related to this module / have access
func (module *Module) getRelatedRoleIds(ctx context.Context) ([]int, error) {
	// cache???
	var roleIds []int
	db := config.GetDB()

	err := db.WithContext(ctx).Model(&RoleModule{}).Select("role_id").
		Where("business_id = ? AND module_id = ?", module.BusinessId, module.ID).Scan(&roleIds).Error
	if err != nil {
		return nil, err
	}
	return roleIds, nil
}

func (input *NewModule) validate(ctx context.Context, id int) error {
	if id == 0 {
		// if module is to be created
		// check businessId exists
		if err := utils.ValidateResourceId[Business](ctx, "", input.BusinessId); err != nil {
			return errors.New("business not found")
		}
	}
	// name
	if err := utils.ValidateUnique[Module](ctx, input.BusinessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func CreateModule(ctx context.Context, input *NewModule) (*Module, error) {

	// ONLY ADMIN can access
	db := config.GetDB()

	// validate module name
	if err := input.validate(ctx, 0); err != nil {
		return nil, err
	}

	module := Module{
		Name:       input.Name,
		BusinessId: input.BusinessId,
		Actions:    input.Actions,
	}

	// create module
	tx := db.Begin()
	err := tx.WithContext(ctx).Create(&module).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// caching
	if err := module.RemoveAllRedis(); err != nil {
		tx.Rollback()
		return nil, err
	}

	return &module, tx.Commit().Error
}

func UpdateModule(ctx context.Context, id int, input *NewModule) (*Module, error) {

	// only admin can access
	db := config.GetDB()
	// check exists
	var count int64
	if err := db.WithContext(ctx).Where("business_id = ? AND id = ?", input.BusinessId, id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count <= 0 {
		return nil, utils.ErrorRecordNotFound
	}

	if err := input.validate(ctx, id); err != nil {
		return nil, err
	}

	module := Module{
		ID:      id,
		Name:    input.Name,
		Actions: input.Actions,
	}

	// update the module
	tx := db.Begin()
	err := tx.WithContext(ctx).Model(&module).Updates(map[string]interface{}{
		"Name":    input.Name,
		"Actions": input.Actions,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// caching
	if err := RemoveRedisBoth(module); err != nil {
		tx.Rollback()
		return nil, err
	}
	// get role ids related to / have access to module
	roleIds, err := module.getRelatedRoleIds(ctx)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// remove from redis
	for _, roleId := range roleIds {
		if err := utils.ClearPathsCache(roleId); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return &module, tx.Commit().Error
}

func DeleteModule(ctx context.Context, id int) (*Module, error) {

	// only admin can access
	db := config.GetDB()
	var result Module

	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	// delete module
	tx := db.Begin()
	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// get related role ids
	roleIds, err := result.getRelatedRoleIds(ctx)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Delete role module
	err = tx.WithContext(ctx).Where("business_id = ? AND module_id = ?", result.BusinessId, id).Delete(&RoleModule{}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// remove from redis
	if err := RemoveRedisBoth(result); err != nil {
		tx.Rollback()
		return nil, err
	}
	for _, roleId := range roleIds {
		if err := utils.ClearPathsCache(roleId); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return &result, tx.Commit().Error
}

func GetModule(ctx context.Context, id int) (*Module, error) {

	// only admin can access
	db := config.GetDB()
	var result Module

	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}
	return &result, nil
}

type AllModule struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Actions string `json:"actions"`
}

type BaseModule struct {
	BaseModuleName string       `json:"baseModuleName"`
	Modules        []*AllModule `json:"modules,omitempty"`
}

func GetModules(ctx context.Context, name *string) ([]*BaseModule, error) {

	allModules, err := ListAllResource[Module, AllModule](ctx, "name")
	if err != nil {
		return nil, err
	}

	// constructs BaseModule[]
	const (
		OthersModule int = iota
		DashboardModule
		FileUploadModule
		ProductsModule
		SalesModule
		PurchasesModule
		AccountantModule
		SettingsModule
		Report_BusinessOverview
		Report_Accountant
		Report_Sales
		Report_Inventory
		Report_Payable
		Report_Receivable
		Report_Purchase
		Report_PaymentReceived
		Report_PaymentMade
		Report_Expense
		Report_Currency
	)
	results := []*BaseModule{
		{
			BaseModuleName: "Others",
		},
		{
			BaseModuleName: "Dashboard",
		},
		{
			BaseModuleName: "FileUpload",
		},
		{
			BaseModuleName: "Products",
		},
		{
			BaseModuleName: "Sales",
		},
		{
			BaseModuleName: "Purchases",
		},
		{
			BaseModuleName: "Accountants",
		},
		{
			BaseModuleName: "Settings",
		},
		{
			BaseModuleName: "Reports:BusinessOverview",
		},
		{
			BaseModuleName: "Reports:Accountant",
		},
		{
			BaseModuleName: "Reports:Sales",
		},
		{
			BaseModuleName: "Reports:Inventory",
		},
		{
			BaseModuleName: "Reports:Payable",
		},
		{
			BaseModuleName: "Reports:Receivable",
		},
		{
			BaseModuleName: "Reports:Purchase",
		},
		{
			BaseModuleName: "Reports:PaymentReceived",
		},
		{
			BaseModuleName: "Reports:PaymentMade",
		},
		{
			BaseModuleName: "Reports:Expense",
		},
		{
			BaseModuleName: "Reports:Currency",
		},
	}

	moduleNameToBase := map[string]int{
		"Account":                          AccountantModule,
		"Accounting":                       AccountantModule,
		"BankingAccount":                   AccountantModule,
		"BankingTransaction":               AccountantModule,
		"Journal":                          AccountantModule,
		"MoneyAccount":                     AccountantModule,
		"Refund":                           AccountantModule,
		"TransactionLocking":               AccountantModule,
		"TransactionLockingRecord":         AccountantModule,
		"TopExpense":                       DashboardModule,
		"TotalCashFlow":                    DashboardModule,
		"TotalIncomeExpense":               DashboardModule,
		"TotalPayableReceivable":           DashboardModule,
		"Image":                            FileUploadModule,
		"Attachment":                       FileUploadModule,
		"File":                             FileUploadModule,
		"Comment":                          OthersModule,
		"State":                            OthersModule,
		"Module":                           OthersModule,
		"Township":                         OthersModule,
		"Product":                          ProductsModule,
		"ProductVariant":                   ProductsModule,
		"ProductGroup":                     ProductsModule,
		"InventoryAdjustment":              ProductsModule,
		"TransferOrder":                    ProductsModule,
		"OpeningStockGroup":                ProductsModule,
		"ProductCategory":                  ProductsModule,
		"ProductModifier":                  ProductsModule,
		"AvailableStocks":                  ProductsModule,
		"ClosingInventory":                 ProductsModule,
		"ProductUnit":                      ProductsModule,
		"ProductTransactions":              ProductsModule,
		"Supplier":                         PurchasesModule,
		"PurchaseOrder":                    PurchasesModule,
		"Bill":                             PurchasesModule,
		"SupplierPayment":                  PurchasesModule,
		"Expense":                          PurchasesModule,
		"SupplierCredit":                   PurchasesModule,
		"SupplierCreditBill":               PurchasesModule,
		"PaymentsMade":                     PurchasesModule,
		"RecurringBill":                    PurchasesModule,
		"UnusedSupplierCreditAdvances":     PurchasesModule,
		"UnusedSupplierCredits":            PurchasesModule,
		"SupplierApplyCredit":              PurchasesModule,
		"SupplierApplyToBill":              PurchasesModule,
		"AccountTransactionReport":         Report_Accountant,
		"AccountTypeSummaryReport":         Report_Accountant,
		"DetailedGeneralLedgerReport":      Report_Accountant,
		"GeneralLedgerReport":              Report_Accountant,
		"JournalReport":                    Report_Accountant,
		"TrialBalanceReport":               Report_Accountant,
		"AccountJournalTransactions":       Report_Accountant,
		"ProfitAndLossReport":              Report_BusinessOverview,
		"CashFlowReport":                   Report_BusinessOverview,
		"MovementOfEquityReport":           Report_BusinessOverview,
		"BalanceSheetReport":               Report_BusinessOverview,
		"RealisedExchangeGainLossReport":   Report_Currency,
		"UnrealisedExchangeGainLossReport": Report_Currency,
		"ExpenseDetailReport":              Report_Expense,
		"ExpenseByCategory":                Report_Expense,
		"ExpenseSummaryByCategory":         Report_Expense,
		"InventorySummaryReport":           Report_Inventory,
		"StockSummaryReport":               Report_Inventory,
		"InventoryValuationSummaryReport":  Report_Inventory,
		"InventoryValuation":               Report_Inventory,
		"WarehouseInventoryReport":         Report_Inventory,
		"ProductSalesReport":               Report_Inventory,
		"APAgingDetailReport":              Report_Payable,
		"APAgingSummaryReport":             Report_Payable,
		"PurchaseOrderDetailReport":        Report_Payable,
		"PayableDetailReport":              Report_Payable,
		"PayableSummaryReport":             Report_Payable,
		"BillDetailReport":                 Report_Payable,
		"SupplierBalancesReport":           Report_Payable,
		"SupplierBalanceSummaryReport":     Report_Payable,
		"PaymentMade":                      Report_PaymentMade,
		"SupplierRefundHistoryReport":      Report_PaymentMade,
		"SupplierCreditDetailReport":       Report_PaymentMade,
		"SupplierCreditDetailsReport":      Report_PaymentMade,
		"PaymentsReceived":                 Report_PaymentReceived,
		"CustomerRefundHistoryReport":      Report_PaymentReceived,
		"CreditNoteDetailsReport":          Report_PaymentReceived,
		"PurchasesBySupplierReport":        Report_Purchase,
		"PurchasesByCustomerReport":        Report_Purchase,
		"PurchasesByProductReport":         Report_Purchase,
		"ARAgingDetailReport":              Report_Receivable,
		"ARAgingSummaryReport":             Report_Receivable,
		"SalesOrderDetailReport":           Report_Receivable,
		"ReceivableDetailReport":           Report_Receivable,
		"ReceivableSummaryReport":          Report_Receivable,
		"SalesInvoiceDetailReport":         Report_Receivable,
		"CustomerBalancesReport":           Report_Receivable,
		"CustomerBalanceSummaryReport":     Report_Receivable,
		"SalesByCustomerReport":            Report_Sales,
		"SalesByProductReport":             Report_Sales,
		"SalesBySalesPersonReport":         Report_Sales,
		"SalesOrder":                       SalesModule,
		"SalesPerson":                      SalesModule,
		"SalesInvoice":                     SalesModule,
		"Customer":                         SalesModule,
		"CreditNote":                       SalesModule,
		"CustomerPayment":                  SalesModule,
		"CustomerApplyCredit":              SalesModule,
		"CustomerApplyToInvoice":           SalesModule,
		"CustomerCreditInvoice":            SalesModule,
		"UnusedCustomerCreditAdvances":     SalesModule,
		"UnusedCustomerCredits":            SalesModule,
		"Warehouse":                        SettingsModule,
		"Business":                         SettingsModule,
		"TransactionNumberSeries":          SettingsModule,
		"Currency":                         SettingsModule,
		"TaxGroup":                         SettingsModule,
		"Tax":                              SettingsModule,
		"PaymentMode":                      SettingsModule,
		"DeliveryMethod":                   SettingsModule,
		"Branch":                           SettingsModule,
		"Reason":                           SettingsModule,
		"ShipmentPreference":               SettingsModule,
		"User":                             SettingsModule,
		"UserAccount":                      SettingsModule,
		"OpeningBalance":                   SettingsModule,
		"Role":                             SettingsModule,
		"TaxSetting":                       SettingsModule,
		"OpeningBalanceDetails":            SettingsModule,
		"PosInvoicePayment":                SettingsModule,
		"RoleModule":                       SettingsModule,
		"Document":                         SettingsModule,
	}

	for _, module := range allModules {
		i := moduleNameToBase[utils.UppercaseFirst(module.Name)] // i is 0 (OthersModule) if not found in map
		results[i].Modules = append(results[i].Modules, module)
	}

	return results, nil
}
