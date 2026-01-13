package models

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProductInterface interface {
	GetPurchasePrice() decimal.Decimal
	GetInventoryAccountID() int
	GetId() int
	GetIsBatchTracking() bool
}

func GetProductOrVariant(ctx context.Context, productType string, productId int) (ProductInterface, error) {
	var product ProductInterface
	var err error
	if productType == string(ProductTypeInput) {
		return nil, errors.New("product type is input")
	} else if productType == string(ProductTypeVariant) {
		product, err = GetProductVariant(ctx, productId)
		// } else if productType == string(ProductTypeGroup) {
		// 	product, err = GetProductGroup(ctx, productId)
	} else {
		product, err = GetProduct(ctx, productId)
	}
	if err != nil {
		return nil, err
	}
	return product, nil
}

func (po *PurchaseOrder) AfterCreate(tx *gorm.DB) error {
	ctx := tx.Statement.Context
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("PURCHASE_ORDER") {
		description, err := describeTotalAmountCreated(ctx, "PurchaseOrder", po.CurrencyId, po.OrderTotalAmount)
		if err != nil {
			tx.Rollback()
			return err
		}
		if err := SaveHistoryCreate(tx, po.ID, po, description); err != nil {
			tx.Rollback()
			return err
		}
		return nil
	}
	if po.CurrentStatus == PurchaseOrderStatusConfirmed {

		// lock stock summary with sql
		// fieldValues, _ := po.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, po.BusinessId, po.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, po.BusinessId, "stockLock", "modelHoodsStockSummary.go", "PoAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, poItem := range po.Details {
			if poItem.ProductId > 0 {

				product, err := GetProductOrVariant(ctx, string(poItem.ProductType), poItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryOrderQty(tx, po.BusinessId, po.WarehouseId, poItem.ProductId, string(poItem.ProductType), poItem.BatchNumber, poItem.DetailQty, po.OrderDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	description, err := describeTotalAmountCreated(ctx, "PurchaseOrder", po.CurrencyId, po.OrderTotalAmount)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := SaveHistoryCreate(tx, po.ID, po, description); err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func (po *PurchaseOrder) BeforeUpdate(tx *gorm.DB) error {
	if err := SaveHistoryUpdate(tx, po.ID, po, "Updated PurchaseOrder"); err != nil {
		return err
	}
	return nil
}

func (po *PurchaseOrder) AfterDelete(tx *gorm.DB) error {
	if err := SaveHistoryDelete(tx, po.ID, po, "Deleted PurchaseOrder"); err != nil {
		return err
	}

	return nil
}

func (po *PurchaseOrder) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("PURCHASE_ORDER") {
		return nil
	}

	if oldStatus != string(po.CurrentStatus) {
		ctx := tx.Statement.Context

		// lock stock summary
		// fieldValues, _ := po.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, po.BusinessId, po.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, po.BusinessId, "stockLock", "modelHoodsStockSummary.go", "PoAfterUpdateCurrentStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, poItem := range po.Details {

			if poItem.ProductId > 0 {

				product, err := GetProductOrVariant(ctx, string(poItem.ProductType), poItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					// Handle actions based on the change
					if oldStatus == string(PurchaseOrderStatusDraft) && po.CurrentStatus == PurchaseOrderStatusConfirmed {
						if err := UpdateStockSummaryOrderQty(tx, po.BusinessId, po.WarehouseId, poItem.ProductId, string(poItem.ProductType), poItem.BatchNumber, poItem.DetailQty, po.OrderDate); err != nil {
							tx.Rollback()
							return err
						}
					}
					if (oldStatus == string(PurchaseOrderStatusConfirmed) && po.CurrentStatus == PurchaseOrderStatusDraft) ||
						(oldStatus == string(PurchaseOrderStatusConfirmed) && po.CurrentStatus == PurchaseOrderStatusClosed) ||
						(oldStatus == string(PurchaseOrderStatusConfirmed) && po.CurrentStatus == PurchaseOrderStatusCancelled) {
						if err := UpdateStockSummaryOrderQty(tx, po.BusinessId, po.WarehouseId, poItem.ProductId, string(poItem.ProductType), poItem.BatchNumber, poItem.DetailQty.Neg(), po.OrderDate); err != nil {
							tx.Rollback()
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (poItem *PurchaseOrderDetail) BeforeUpdate(tx *gorm.DB) (err error) {

	ctx := tx.Statement.Context
	if poItem.ProductId > 0 {
		product, err := GetProductOrVariant(ctx, string(poItem.ProductType), poItem.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}

		if product.GetInventoryAccountID() > 0 {
			var oldQty decimal.Decimal
			// Fetch old status
			if err := tx.Model(&PurchaseOrderDetail{}).Where("id = ?", poItem.ID).Select("detail_qty").First(&oldQty).Error; err != nil {
				return err
			}

			// Check if qty has changed
			if poItem.DetailQty != oldQty {
				var purchaseOrder PurchaseOrder
				if err := tx.Model(&PurchaseOrder{}).Where("id = ?", poItem.PurchaseOrderId).First(&purchaseOrder).Error; err != nil {
					tx.Rollback()
					return err
				}
				err := utils.BusinessLock(ctx, purchaseOrder.BusinessId, "stockLock", "modelHoodsStockSummary.go", "PoDetailBeforeUpdate")
				if err != nil {
					tx.Rollback()
					return err
				}
				if purchaseOrder.CurrentStatus == PurchaseOrderStatusConfirmed {
					if err := UpdateStockSummaryOrderQty(tx, purchaseOrder.BusinessId, purchaseOrder.WarehouseId, poItem.ProductId, string(poItem.ProductType), poItem.BatchNumber, poItem.DetailQty.Sub(oldQty), purchaseOrder.OrderDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	return nil
}

func (bill *Bill) AfterCreate(tx *gorm.DB) (err error) {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("BILL") {
		ctx := tx.Statement.Context
		description, err := describeTotalAmountCreated(ctx, "Bill", bill.CurrencyId, bill.BillTotalAmount)
		if err != nil {
			tx.Rollback()
			return err
		}
		if err := SaveHistoryCreate(tx, bill.ID, bill, description); err != nil {
			tx.Rollback()
			return err
		}
		return nil
	}

	ctx := tx.Statement.Context

	if bill.PurchaseOrderId > 0 && bill.CurrentStatus == BillStatusConfirmed {
		if err := ClosePoStatus(tx, bill); err != nil {
			tx.Rollback()
			return err
		}
	}
	if bill.CurrentStatus == BillStatusConfirmed {

		// lock stock summary
		// fieldValues, _ := bill.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, bill.BusinessId, bill.WarehouseId, fieldValues)
		// lock business
		err := utils.BusinessLock(ctx, bill.BusinessId, "stockLock", "modelHoodsStockSummary.go", "BillAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}
		for _, billItem := range bill.Details {
			if billItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryReceivedQty(tx, bill.BusinessId, bill.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty, bill.BillDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	description, err := describeTotalAmountCreated(ctx, "Bill", bill.CurrencyId, bill.BillTotalAmount)
	if err != nil {
		tx.Rollback()
		return err
	}
	if err := SaveHistoryCreate(tx, bill.ID, bill, description); err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func (b *Bill) AfterDelete(tx *gorm.DB) (err error) {
	if err := SaveHistoryDelete(tx, b.ID, b, "Deleted Bill"); err != nil {
		return err
	}

	return nil
}

func (b *Bill) BeforeUpdate(tx *gorm.DB) error {
	if err := SaveHistoryUpdate(tx, b.ID, b, "Updated Bill"); err != nil {
		return err
	}
	return nil
}

func (bill *Bill) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("BILL") {
		return nil
	}
	if oldStatus != string(bill.CurrentStatus) {
		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := bill.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, bill.BusinessId, bill.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, bill.BusinessId, "stockLock", "modelHoodsStockSummary.go", "BillAfterUpdateCurrentStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, billItem := range bill.Details {
			if billItem.ProductId > 0 {

				product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					// Handle actions based on the change
					if oldStatus == string(BillStatusDraft) && bill.CurrentStatus == BillStatusConfirmed {

						if err := UpdateStockSummaryReceivedQty(tx, bill.BusinessId, bill.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty, bill.BillDate); err != nil {
							tx.Rollback()
							return err
						}
					}
					if (oldStatus == string(BillStatusConfirmed) && bill.CurrentStatus == BillStatusDraft) ||
						(oldStatus == string(BillStatusConfirmed) && bill.CurrentStatus == BillStatusVoid) {
						if err := UpdateStockSummaryReceivedQty(tx, bill.BusinessId, bill.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty.Neg(), bill.BillDate); err != nil {
							tx.Rollback()
							return err
						}
					}

				}
			}
		}
	}
	return nil
}

func (billItem *BillDetail) BeforeUpdate(tx *gorm.DB) (err error) {
	// ctx := tx.Statement.Context
	// if billItem.ProductId > 0 {
	// 	product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
	// 	if err != nil {
	// 		tx.Rollback()
	// 		return err
	// 	}

	// 	if product.GetInventoryAccountID() > 0 {
	// 		var oldQty decimal.Decimal
	// 		// Fetch old status
	// 		if err := tx.Model(&BillDetail{}).Where("id = ?", billItem.ID).Select("detail_qty").First(&oldQty).Error; err != nil {
	// 			return err
	// 		}
	// 		// Check if qty has changed
	// 		if billItem.DetailQty != oldQty {
	// 			var bill Bill
	// 			if err := tx.Model(&Bill{}).Where("id = ?", billItem.BillId).First(&bill).Error; err != nil {
	// 				tx.Rollback()
	// 				return err
	// 			}

	// 			// lock business
	// 			err := utils.BusinessLock(ctx, bill.BusinessId, "stockLock", "modelHoodsStockSummary.go", "BillDetailBeforeUpdate")
	// 			if err != nil {
	// 				tx.Rollback()
	// 				return err
	// 			}

	// 			if bill.CurrentStatus == BillStatusConfirmed {
	// 				if err := UpdateStockSummaryReceivedQty(tx, bill.BusinessId, bill.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty.Sub(oldQty), bill.BillDate); err != nil {
	// 					return err
	// 				}
	// 			}
	// 		}
	// 	}
	// }
	return nil
}

func ClosePoStatus(tx *gorm.DB, bill *Bill) error {
	if bill.PurchaseOrderNumber != "" && len(bill.PurchaseOrderNumber) > 0 {

		ctx := tx.Statement.Context

		var purchaseOrder PurchaseOrder
		err := tx.Preload("Details").Where("business_id = ? AND id = ?", bill.BusinessId, bill.PurchaseOrderId).First(&purchaseOrder).Error
		if err != nil {
			return err
		}

		oldStatus := purchaseOrder.CurrentStatus

		_, err = ChangePoCurrentStatus(tx, ctx, bill.BusinessId, bill.PurchaseOrderId)
		if err != nil {
			tx.Rollback()
			return err
		}

		// lock stock summary
		// fieldValues, _ := purchaseOrder.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, purchaseOrder.BusinessId, purchaseOrder.WarehouseId, fieldValues)

		// lock business
		err = utils.BusinessLock(ctx, purchaseOrder.BusinessId, "stockLock", "modelHoodsStockSummary.go", "ClosePoStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, billItem := range bill.Details {
			if billItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {

					if bill.PurchaseOrderId > 0 {
						var poDetail PurchaseOrderDetail
						var err error
						if billItem.PurchaseOrderItemId > 0 {
							err = tx.Where("id = ?", billItem.PurchaseOrderItemId).First(&poDetail).Error
							if err != nil {
								tx.Rollback()
								return err
							}

							if PurchaseOrderStatus(oldStatus) == PurchaseOrderStatusPartiallyBilled ||
								(PurchaseOrderStatus(oldStatus) == PurchaseOrderStatusConfirmed && bill.CurrentStatus == BillStatusConfirmed) {
								if err := UpdateStockSummaryOrderQty(tx, purchaseOrder.BusinessId, purchaseOrder.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty.Neg(), purchaseOrder.OrderDate); err != nil {
									tx.Rollback()
									return err
								}
							}

							if PurchaseOrderStatus(oldStatus) == PurchaseOrderStatusDraft {
								if bill.CurrentStatus == BillStatusConfirmed {
									if err := UpdateStockSummaryOrderQty(tx, purchaseOrder.BusinessId, purchaseOrder.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, poDetail.DetailQty.Sub(billItem.DetailQty), purchaseOrder.OrderDate); err != nil {
										tx.Rollback()
										return err
									}
								} else {
									if err := UpdateStockSummaryOrderQty(tx, purchaseOrder.BusinessId, purchaseOrder.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, poDetail.DetailQty, purchaseOrder.OrderDate); err != nil {
										tx.Rollback()
										return err
									}
								}

							}
						}

					}
				}
			}
		}
	}
	return nil
}

func (sale *SalesOrder) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("SALES_ORDER") {
		return nil
	}
	if oldStatus != string(sale.CurrentStatus) {
		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := sale.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, sale.BusinessId, sale.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, sale.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SaleOrderAfterUpdateCurrentStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, saleItem := range sale.Details {
			if saleItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					// Handle actions based on the change
					if oldStatus == string(SalesOrderStatusDraft) && sale.CurrentStatus == SalesOrderStatusConfirmed {
						if err := UpdateStockSummaryCommittedQty(tx, sale.BusinessId, sale.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty, sale.OrderDate); err != nil {
							tx.Rollback()
							return err
						}
					}
					if (oldStatus == string(SalesOrderStatusConfirmed) && sale.CurrentStatus == SalesOrderStatusDraft) ||
						(oldStatus == string(SalesOrderStatusConfirmed) && sale.CurrentStatus == SalesOrderStatusCancelled) ||
						(oldStatus == string(SalesOrderStatusConfirmed) && sale.CurrentStatus == SalesOrderStatusClosed) {
						if err := UpdateStockSummaryCommittedQty(tx, sale.BusinessId, sale.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty.Neg(), sale.OrderDate); err != nil {
							tx.Rollback()
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (saleItem *SalesOrderDetail) BeforeUpdate(tx *gorm.DB) (err error) {

	ctx := tx.Statement.Context
	if saleItem.ProductId > 0 {
		product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}

		if product.GetInventoryAccountID() > 0 {
			var oldQty decimal.Decimal
			// Fetch old status
			if err := tx.Model(&SalesOrderDetail{}).Where("id = ?", saleItem.ID).Select("detail_qty").First(&oldQty).Error; err != nil {
				tx.Rollback()
				return err
			}
			// Check if qty has changed
			if saleItem.DetailQty != oldQty {
				var saleOrder SalesOrder
				if err := tx.Model(&SalesOrder{}).Where("id = ?", saleItem.SalesOrderId).First(&saleOrder).Error; err != nil {
					tx.Rollback()
					return err
				}
				// lock business
				err := utils.BusinessLock(ctx, saleOrder.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SaleOrderDetailBeforeUpdate")
				if err != nil {
					tx.Rollback()
					return err
				}

				if saleOrder.CurrentStatus == SalesOrderStatusConfirmed {
					if err := UpdateStockSummaryCommittedQty(tx, saleOrder.BusinessId, saleOrder.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty.Sub(oldQty), saleOrder.OrderDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}
	return nil
}

func (sale *SalesInvoice) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	// Keeping this method as a no-op avoids accidental double-application.
	if config.UseStockCommandsFor("SALES_INVOICE") {
		return nil
	}

	if oldStatus != string(sale.CurrentStatus) {
		// if oldStatus == string(SalesInvoiceStatusDraft) && sale.CurrentStatus == SalesInvoiceStatusConfirmed {
		// 	if err := CloseSalesOrderStatus(tx, sale); err != nil {
		// 		tx.Rollback()
		// 		return err
		// 	}
		// }
		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := sale.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, sale.BusinessId, sale.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, sale.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SaleInvoiceAfterUpdateCurrentStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, saleItem := range sale.Details {
			if saleItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					// Handle actions based on the change
					if oldStatus == string(SalesInvoiceStatusDraft) && sale.CurrentStatus == SalesInvoiceStatusConfirmed {
						if err := UpdateStockSummarySaleQty(tx, sale.BusinessId, sale.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty, sale.InvoiceDate); err != nil {
							tx.Rollback()
							return err
						}
					}
					if (oldStatus == string(SalesInvoiceStatusConfirmed) && sale.CurrentStatus == SalesInvoiceStatusDraft) ||
						(oldStatus == string(SalesInvoiceStatusConfirmed) && sale.CurrentStatus == SalesInvoiceStatusVoid) {
						if err := UpdateStockSummarySaleQty(tx, sale.BusinessId, sale.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty.Neg(), sale.InvoiceDate); err != nil {
							tx.Rollback()
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (saleItem *SalesInvoiceDetail) BeforeUpdate(tx *gorm.DB) (err error) {
	// ctx := tx.Statement.Context
	// if saleItem.ProductId > 0 {
	// 	product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
	// 	if err != nil {
	// 		tx.Rollback()
	// 		return err
	// 	}

	// 	if product.GetInventoryAccountID() > 0 {
	// 		var oldQty decimal.Decimal
	// 		// Fetch old status
	// 		if err := tx.Model(&SalesInvoiceDetail{}).Where("id = ?", saleItem.ID).Select("detail_qty").First(&oldQty).Error; err != nil {
	// 			return err
	// 		}
	// 		// Check if qty has changed
	// 		if saleItem.DetailQty != oldQty {
	// 			var invoice SalesInvoice
	// 			if err := tx.Model(&SalesInvoice{}).Where("id = ?", saleItem.SalesInvoiceId).First(&invoice).Error; err != nil {
	// 				tx.Rollback()
	// 				return err
	// 			}

	// 			// lock business
	// 			err := utils.BusinessLock(ctx, invoice.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SaleInvoiceDetailBeforeUpdate")
	// 			if err != nil {
	// 				tx.Rollback()
	// 				return err
	// 			}
	// 			if invoice.CurrentStatus == SalesInvoiceStatusConfirmed {
	// 				if err := UpdateStockSummarySaleQty(tx, invoice.BusinessId, invoice.WarehouseId, saleItem.ProductId, string(saleItem.ProductType), saleItem.BatchNumber, saleItem.DetailQty.Sub(oldQty), invoice.InvoiceDate); err != nil {
	// 					return err
	// 				}
	// 			}
	// 		}
	// 	}
	// }
	return nil
}

func CloseSalesOrderStatus(tx *gorm.DB, saleInvoice *SalesInvoice) error {
	if saleInvoice.OrderNumber != "" && len(saleInvoice.OrderNumber) > 0 {

		ctx := tx.Statement.Context

		var saleOrder SalesOrder

		err := tx.Preload("Details").Where("business_id = ? AND order_number = ?", saleInvoice.BusinessId, saleInvoice.OrderNumber).First(&saleOrder).Error
		if err != nil {
			tx.Rollback()
			return err
		}

		oldStatus := saleOrder.CurrentStatus

		_, err = ChangeSaleOrderCurrentStatus(tx, ctx, saleInvoice.BusinessId, saleInvoice.SalesOrderId)
		if err != nil {
			tx.Rollback()
			return err
		}

		// lock stock summary
		// fieldValues, _ := saleOrder.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, saleOrder.BusinessId, saleOrder.WarehouseId, fieldValues)
		// lock business
		err = utils.BusinessLock(ctx, saleInvoice.BusinessId, "stockLock", "modelHoodsStockSummary.go", "CloseSalesOrderStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, invoiceItem := range saleInvoice.Details {
			if invoiceItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(invoiceItem.ProductType), invoiceItem.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {

					if saleInvoice.SalesOrderId > 0 {
						var saleOrderDetail SalesOrderDetail
						var err error
						if invoiceItem.SalesOrderItemId > 0 {
							err = tx.Where("id = ?", invoiceItem.SalesOrderItemId).First(&saleOrderDetail).Error
							if err != nil {
								tx.Rollback()
								return err
							}

							if SalesOrderStatus(oldStatus) == SalesOrderStatusPartiallyInvoiced ||
								(SalesOrderStatus(oldStatus) == SalesOrderStatusConfirmed && saleInvoice.CurrentStatus == SalesInvoiceStatusConfirmed) {
								if err := UpdateStockSummaryCommittedQty(tx, saleOrder.BusinessId, saleOrder.WarehouseId, invoiceItem.ProductId, string(invoiceItem.ProductType), invoiceItem.BatchNumber, invoiceItem.DetailQty.Neg(), saleOrder.OrderDate); err != nil {
									tx.Rollback()
									return err
								}
							}

							if SalesOrderStatus(oldStatus) == SalesOrderStatusDraft {
								if saleInvoice.CurrentStatus == SalesInvoiceStatusConfirmed {
									if err := UpdateStockSummaryCommittedQty(tx, saleOrder.BusinessId, saleOrder.WarehouseId, invoiceItem.ProductId, string(invoiceItem.ProductType), invoiceItem.BatchNumber, saleOrderDetail.DetailQty.Sub(invoiceItem.DetailQty), saleOrder.OrderDate); err != nil {
										tx.Rollback()
										return err
									}
								} else {
									if err := UpdateStockSummaryCommittedQty(tx, saleOrder.BusinessId, saleOrder.WarehouseId, invoiceItem.ProductId, string(invoiceItem.ProductType), invoiceItem.BatchNumber, saleOrderDetail.DetailQty, saleOrder.OrderDate); err != nil {
										tx.Rollback()
										return err
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (s *SupplierCredit) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("SUPPLIER_CREDIT") {
		return nil
	}

	if oldStatus != string(s.CurrentStatus) {
		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := s.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, s.BusinessId, s.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, s.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SupplierCreditAfterUpdateCurrentStatus")
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
					// Handle actions based on the change
					if oldStatus == string(SupplierCreditStatusDraft) && s.CurrentStatus == SupplierCreditStatusConfirmed {

						if err := UpdateStockSummaryReceivedQty(tx, s.BusinessId, s.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Neg(), s.SupplierCreditDate); err != nil {
							tx.Rollback()
							return err
						}
					}
					if (oldStatus == string(SupplierCreditStatusConfirmed) && s.CurrentStatus == SupplierCreditStatusDraft) ||
						(oldStatus == string(SupplierCreditStatusConfirmed) && s.CurrentStatus == SupplierCreditStatusClosed) ||
						(oldStatus == string(SupplierCreditStatusConfirmed) && s.CurrentStatus == SupplierCreditStatusVoid) {
						if err := UpdateStockSummaryReceivedQty(tx, s.BusinessId, s.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty, s.SupplierCreditDate); err != nil {
							tx.Rollback()
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

// func (creditItem *SupplierCreditDetail) BeforeUpdate(tx *gorm.DB) (err error) {
// ctx := tx.Statement.Context
// if creditItem.ProductId > 0 {
// 	product, err := GetProductOrVariant(ctx, string(creditItem.ProductType), creditItem.ProductId)
// 	if err != nil {
// 		tx.Rollback()
// 		return err
// 	}

// 	if product.GetInventoryAccountID() > 0 {
// 		var oldQty decimal.Decimal
// 		// Fetch old status
// 		if err := tx.Model(&SupplierCreditDetail{}).Where("id = ?", creditItem.ID).Select("detail_qty").First(&oldQty).Error; err != nil {
// 			return err
// 		}
// 		// Check if qty has changed
// 		if creditItem.DetailQty != oldQty {
// 			var supplierCredit SupplierCredit
// 			if err := tx.Model(&SupplierCredit{}).Where("id = ?", creditItem.SupplierCreditId).First(&supplierCredit).Error; err != nil {
// 				tx.Rollback()
// 				return err
// 			}

// 			// lock business
// 			err := utils.BusinessLock(ctx, supplierCredit.BusinessId, "stockLock", "modelHoodsStockSummary.go", "SupplierCreditDetailBeforeUpdate")
// 			if err != nil {
// 				tx.Rollback()
// 				return err
// 			}
// 			if supplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {
// 				if err := UpdateStockSummaryReceivedQty(tx, supplierCredit.BusinessId, supplierCredit.WarehouseId, creditItem.ProductId, string(creditItem.ProductType), creditItem.BatchNumber, creditItem.DetailQty.Sub(oldQty), supplierCredit.SupplierCreditDate); err != nil {
// 					return err
// 				}
// 			}
// 		}

// 	}
// }
// 	return nil
// }

func (creditNote *CreditNote) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("CREDIT_NOTE") {
		return nil
	}

	if oldStatus != string(creditNote.CurrentStatus) {

		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := creditNote.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, creditNote.BusinessId, creditNote.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, creditNote.BusinessId, "stockLock", "modelHoodsStockSummary.go", "CreditNoteAfterUpdateCurrentStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, item := range creditNote.Details {
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					// Handle actions based on the change
					if oldStatus == string(CreditNoteStatusDraft) && creditNote.CurrentStatus == CreditNoteStatusConfirmed {
						if err := UpdateStockSummaryReceivedQty(tx, creditNote.BusinessId, creditNote.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty, creditNote.CreditNoteDate); err != nil {
							tx.Rollback()
							return err
						}
					}
					if (oldStatus == string(CreditNoteStatusConfirmed) && creditNote.CurrentStatus == CreditNoteStatusDraft) ||
						(oldStatus == string(CreditNoteStatusConfirmed) && creditNote.CurrentStatus == CreditNoteStatusVoid) {
						if err := UpdateStockSummaryReceivedQty(tx, creditNote.BusinessId, creditNote.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Neg(), creditNote.CreditNoteDate); err != nil {
							tx.Rollback()
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

// func (item *CreditNoteDetail) BeforeUpdate(tx *gorm.DB) (err error) {
// 	ctx := tx.Statement.Context
// 	if item.ProductId > 0 {
// 		product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
// 		if err != nil {
// 			tx.Rollback()
// 			return err
// 		}

// 		if product.GetInventoryAccountID() > 0 {
// 			var oldQty decimal.Decimal
// 			// Fetch old status
// 			if err := tx.Model(&CreditNoteDetail{}).Where("id = ?", item.ID).Select("detail_qty").First(&oldQty).Error; err != nil {
// 				return err
// 			}
// 			// Check if qty has changed
// 			if item.DetailQty != oldQty {
// 				var creditNote CreditNote
// 				if err := tx.Model(&CreditNote{}).Where("id = ?", item.CreditNoteId).First(&creditNote).Error; err != nil {
// 					tx.Rollback()
// 					return err
// 				}
// 				// lock business
// 				err := utils.BusinessLock(ctx, creditNote.BusinessId, "stockLock", "modelHoodsStockSummary.go", "CreditNoteDetailBeforeUpdate")
// 				if err != nil {
// 					tx.Rollback()
// 					return err
// 				}
// 				if creditNote.CurrentStatus == CreditNoteStatusConfirmed {
// 					if err := UpdateStockSummaryReceivedQty(tx, creditNote.BusinessId, creditNote.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Sub(oldQty), creditNote.CreditNoteDate); err != nil {
// 						return err
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

func (inventoryAdjustment *InventoryAdjustment) AfterCreate(tx *gorm.DB) (err error) {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	// Keeping history is still important.
	if config.UseStockCommandsFor("INVENTORY_ADJUSTMENT") {
		if err := SaveHistoryCreate(tx, inventoryAdjustment.ID, inventoryAdjustment, "Created InventoryAdjustment"); err != nil {
			return err
		}
		return nil
	}

	ctx := tx.Statement.Context
	if inventoryAdjustment.CurrentStatus == InventoryAdjustmentStatusAdjusted && inventoryAdjustment.AdjustmentType == InventoryAdjustmentTypeQuantity {

		// lock stock summary
		// fieldValues, _ := inventoryAdjustment.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, inventoryAdjustment.BusinessId, inventoryAdjustment.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, inventoryAdjustment.BusinessId, "stockLock", "modelHoodsStockSummary.go", "InventoryAdjustmentAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, detail := range inventoryAdjustment.Details {
			if detail.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(detail.ProductType), detail.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					if detail.AdjustedValue.GreaterThan(decimal.NewFromFloat(0)) {
						if err := UpdateStockSummaryAdjustedQtyIn(tx, inventoryAdjustment.BusinessId, inventoryAdjustment.WarehouseId, detail.ProductId, string(detail.ProductType), detail.BatchNumber, detail.AdjustedValue, inventoryAdjustment.AdjustmentDate); err != nil {
							tx.Rollback()
							return err
						}
					} else {
						if err := UpdateStockSummaryAdjustedQtyOut(tx, inventoryAdjustment.BusinessId, inventoryAdjustment.WarehouseId, detail.ProductId, string(detail.ProductType), detail.BatchNumber, detail.AdjustedValue, inventoryAdjustment.AdjustmentDate); err != nil {
							tx.Rollback()
							return err
						}
					}

				}
			}
		}
	}

	if err := SaveHistoryCreate(tx, inventoryAdjustment.ID, inventoryAdjustment, "Created InventoryAdjustment"); err != nil {
		return err
	}

	return nil
}

func (invAdj *InventoryAdjustment) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("INVENTORY_ADJUSTMENT") {
		return nil
	}

	if oldStatus != string(invAdj.CurrentStatus) {

		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := invAdj.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, invAdj.BusinessId, invAdj.WarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, invAdj.BusinessId, "stockLock", "modelHoodsStockSummary.go", "InventoryAdjustmentAfterUpdateCurrentStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, item := range invAdj.Details {
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					// Handle actions based on the change
					if item.AdjustedValue.GreaterThan(decimal.NewFromFloat(0)) {
						if err := UpdateStockSummaryAdjustedQtyIn(tx, invAdj.BusinessId, invAdj.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.AdjustedValue, invAdj.AdjustmentDate); err != nil {
							tx.Rollback()
							return err
						}
					} else {
						if err := UpdateStockSummaryAdjustedQtyOut(tx, invAdj.BusinessId, invAdj.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.AdjustedValue, invAdj.AdjustmentDate); err != nil {
							tx.Rollback()
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (invAdjDetail *InventoryAdjustmentDetail) BeforeUpdate(tx *gorm.DB) (err error) {

	ctx := tx.Statement.Context
	var invAdj InventoryAdjustment
	if err := tx.Model(&InventoryAdjustment{}).Where("id = ?", invAdjDetail.InventoryAdjustmentId).First(&invAdj).Error; err != nil {
		tx.Rollback()
		return err
	}
	if invAdjDetail.ProductId > 0 && invAdj.CurrentStatus == InventoryAdjustmentStatusAdjusted && invAdj.AdjustmentType == InventoryAdjustmentTypeQuantity {

		product, err := GetProductOrVariant(ctx, string(invAdjDetail.ProductType), invAdjDetail.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}

		if product.GetInventoryAccountID() > 0 {
			var oldQty decimal.Decimal
			// Fetch old status
			if err := tx.Model(&InventoryAdjustmentDetail{}).Where("id = ?", invAdjDetail.ID).Select("adjusted_value").First(&oldQty).Error; err != nil {
				tx.Rollback()
				return err
			}
			// Check if qty has changed
			if invAdjDetail.AdjustedValue != oldQty {

				// lock business
				err := utils.BusinessLock(ctx, invAdj.BusinessId, "stockLock", "modelHoodsStockSummary.go", "InventoryAdjustmentDetailBeforeUpdate")
				if err != nil {
					tx.Rollback()
					return err
				}

				if invAdjDetail.AdjustedValue.GreaterThan(decimal.NewFromFloat(0)) {
					if err := UpdateStockSummaryAdjustedQtyIn(tx, invAdj.BusinessId, invAdj.WarehouseId, invAdjDetail.ProductId, string(invAdjDetail.ProductType), invAdjDetail.BatchNumber, invAdjDetail.AdjustedValue.Sub(oldQty), invAdj.AdjustmentDate); err != nil {
						tx.Rollback()
						return err
					}
				} else {
					if err := UpdateStockSummaryAdjustedQtyOut(tx, invAdj.BusinessId, invAdj.WarehouseId, invAdjDetail.ProductId, string(invAdjDetail.ProductType), invAdjDetail.BatchNumber, invAdjDetail.AdjustedValue.Sub(oldQty), invAdj.AdjustmentDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}
	return nil
}

func (transferOrder *TransferOrder) AfterCreate(tx *gorm.DB) (err error) {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	// Keeping history is still important.
	if config.UseStockCommandsFor("TRANSFER_ORDER") {
		if err := SaveHistoryCreate(tx, transferOrder.ID, transferOrder, "Created TransferOrder"); err != nil {
			return err
		}
		return nil
	}

	ctx := tx.Statement.Context
	if transferOrder.CurrentStatus == TransferOrderStatusConfirmed {

		// lock stock summary
		// fieldValues, _ := transferOrder.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, transferOrder.BusinessId, transferOrder.SourceWarehouseId, fieldValues)
		// _ = BulkLockStockSummary(tx, transferOrder.BusinessId, transferOrder.DestinationWarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, transferOrder.BusinessId, "stockLock", "modelHoodsStockSummary.go", "TransferOrderAfterCreate")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, detail := range transferOrder.Details {
			if detail.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(detail.ProductType), detail.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryTransferQtyOut(tx, transferOrder.BusinessId, transferOrder.SourceWarehouseId, detail.ProductId, string(detail.ProductType), detail.BatchNumber, detail.TransferQty.Neg(), transferOrder.TransferDate); err != nil {
						tx.Rollback()
						return err
					}
					if err := UpdateStockSummaryTransferQtyIn(tx, transferOrder.BusinessId, transferOrder.DestinationWarehouseId, detail.ProductId, string(detail.ProductType), detail.BatchNumber, detail.TransferQty, transferOrder.TransferDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}

	if err := SaveHistoryCreate(tx, transferOrder.ID, transferOrder, "Created TransferOrder"); err != nil {
		return err
	}

	return nil
}

func (transferOrder *TransferOrder) AfterUpdateCurrentStatus(tx *gorm.DB, oldStatus string) error {
	// When enabled, inventory side-effects are owned by explicit command handlers (not model hooks).
	if config.UseStockCommandsFor("TRANSFER_ORDER") {
		return nil
	}

	if oldStatus != string(transferOrder.CurrentStatus) && oldStatus == string(TransferOrderStatusDraft) && transferOrder.CurrentStatus == TransferOrderStatusConfirmed {

		ctx := tx.Statement.Context
		// lock stock summary
		// fieldValues, _ := transferOrder.GetFieldValues(tx)
		// _ = BulkLockStockSummary(tx, transferOrder.BusinessId, transferOrder.SourceWarehouseId, fieldValues)
		// _ = BulkLockStockSummary(tx, transferOrder.BusinessId, transferOrder.DestinationWarehouseId, fieldValues)

		// lock business
		err := utils.BusinessLock(ctx, transferOrder.BusinessId, "stockLock", "modelHoodsStockSummary.go", "TransferOrderAfterUpdateCurrentStatus")
		if err != nil {
			tx.Rollback()
			return err
		}

		for _, item := range transferOrder.Details {
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return err
				}

				if product.GetInventoryAccountID() > 0 {
					// Handle actions based on the change
					if err := UpdateStockSummaryTransferQtyOut(tx, transferOrder.BusinessId, transferOrder.SourceWarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.TransferQty.Neg(), transferOrder.TransferDate); err != nil {
						tx.Rollback()
						return err
					}
					if err := UpdateStockSummaryTransferQtyIn(tx, transferOrder.BusinessId, transferOrder.DestinationWarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.TransferQty, transferOrder.TransferDate); err != nil {
						tx.Rollback()
						return err
					}
				}
			}
		}
	}
	return nil
}

func (orderDetail *TransferOrderDetail) BeforeUpdate(tx *gorm.DB) (err error) {

	ctx := tx.Statement.Context
	var transferOrder TransferOrder
	if err := tx.Model(&TransferOrder{}).Where("id = ?", orderDetail.TransferOrderId).First(&transferOrder).Error; err != nil {
		tx.Rollback()
		return err
	}
	if orderDetail.ProductId > 0 && transferOrder.CurrentStatus == TransferOrderStatusConfirmed {

		product, err := GetProductOrVariant(ctx, string(orderDetail.ProductType), orderDetail.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}

		if product.GetInventoryAccountID() > 0 {
			var oldQty decimal.Decimal
			// Fetch old status
			if err := tx.Model(&TransferOrderDetail{}).Where("id = ?", orderDetail.ID).Select("transfer_qty").First(&oldQty).Error; err != nil {
				tx.Rollback()
				return err
			}
			// Check if qty has changed
			if orderDetail.TransferQty != oldQty {
				// lock business
				err := utils.BusinessLock(ctx, transferOrder.BusinessId, "stockLock", "modelHoodsStockSummary.go", "TransferOrderDetailBeforeUpdate")
				if err != nil {
					tx.Rollback()
					return err
				}
				if err := UpdateStockSummaryTransferQtyOut(tx, transferOrder.BusinessId, transferOrder.SourceWarehouseId, orderDetail.ProductId, string(orderDetail.ProductType), orderDetail.BatchNumber, orderDetail.TransferQty.Sub(oldQty).Neg(), transferOrder.TransferDate); err != nil {
					tx.Rollback()
					return err
				}
				if err := UpdateStockSummaryTransferQtyIn(tx, transferOrder.BusinessId, transferOrder.DestinationWarehouseId, orderDetail.ProductId, string(orderDetail.ProductType), orderDetail.BatchNumber, orderDetail.TransferQty.Sub(oldQty), transferOrder.TransferDate); err != nil {
					tx.Rollback()
					return err
				}
			}
		}
	}
	return nil
}
