package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type openingRowScan struct {
	BusinessId     string             `gorm:"column:business_id"`
	ProductId      int                `gorm:"column:product_id"`
	ProductType    models.ProductType `gorm:"column:product_type"`
	WarehouseId    int                `gorm:"column:warehouse_id"`
	BatchNumber    string             `gorm:"column:batch_number"`
	Qty            decimal.Decimal    `gorm:"column:qty"`
	UnitValue      decimal.Decimal    `gorm:"column:unit_value"`
	ProductGroupId int                `gorm:"column:product_group_id"`
}

func main() {
	businessId := flag.String("business-id", "", "Business ID to backfill (optional; default = all)")
	dryRun := flag.Bool("dry-run", true, "Print actions without writing")
	fixNullWarehouse := flag.Bool("fix-null-warehouse", true, "Assign missing warehouse_id to default warehouse")
	rebuild := flag.Bool("rebuild", true, "Rebuild inventory after backfill for affected items")
	flag.Parse()

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		panic("database not initialized")
	}
	logger := config.GetLogger()
	if logger == nil {
		logger = logrus.New()
	}

	var businessIds []string
	if strings.TrimSpace(*businessId) != "" {
		businessIds = []string{strings.TrimSpace(*businessId)}
	} else {
		var ids []string
		if err := db.Model(&models.Business{}).Pluck("id", &ids).Error; err != nil {
			panic(err)
		}
		businessIds = ids
	}

	for _, bid := range businessIds {
		if bid == "" {
			continue
		}

		var biz models.Business
		if err := db.Where("id = ?", bid).First(&biz).Error; err != nil {
			logger.WithFields(logrus.Fields{"business_id": bid}).Warn("skip business: not found")
			continue
		}

		var defaultWh models.Warehouse
		if err := db.Where("business_id = ?", bid).Order("id ASC").First(&defaultWh).Error; err != nil {
			logger.WithFields(logrus.Fields{"business_id": bid}).Warn("skip business: no warehouse")
			continue
		}

		if *fixNullWarehouse {
			if *dryRun {
				logger.WithFields(logrus.Fields{"business_id": bid, "warehouse_id": defaultWh.ID}).Info("dry-run: fix NULL/0 warehouse_id")
			} else {
				if err := db.Model(&models.StockHistory{}).
					Where("business_id = ? AND (warehouse_id IS NULL OR warehouse_id = 0)", bid).
					Update("warehouse_id", defaultWh.ID).Error; err != nil {
					panic(err)
				}
			}
		}

		var rows []openingRowScan
		query := `
SELECT
	os.product_id,
	os.product_type,
	os.warehouse_id,
	os.batch_number,
	os.qty,
	os.unit_value,
	os.product_group_id,
	COALESCE(p.business_id, pv.business_id) AS business_id
FROM opening_stocks os
LEFT JOIN products p
	ON os.product_type = 'S'
	AND p.id = os.product_id
LEFT JOIN product_variants pv
	ON os.product_type = 'V'
	AND pv.id = os.product_id
WHERE COALESCE(p.business_id, pv.business_id) = ?
`
		if err := db.Raw(query, bid).Scan(&rows).Error; err != nil {
			panic(err)
		}

		for _, r := range rows {
			if r.BusinessId == "" || r.ProductId == 0 {
				continue
			}
			warehouseId := r.WarehouseId
			if warehouseId <= 0 {
				warehouseId = defaultWh.ID
			}

			refType := models.StockReferenceTypeProductOpeningStock
			refID := r.ProductId
			if r.ProductType == models.ProductTypeVariant && r.ProductGroupId > 0 {
				refType = models.StockReferenceTypeProductGroupOpeningStock
				refID = r.ProductGroupId
			}

			var exists int64
			if err := db.Model(&models.StockHistory{}).
				Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ?",
					r.BusinessId, warehouseId, r.ProductId, r.ProductType, r.BatchNumber).
				Where("reference_type IN ('POS','PGOS','PCOS')").
				Where("is_reversal = 0 AND reversed_by_stock_history_id IS NULL").
				Count(&exists).Error; err != nil {
				panic(err)
			}
			if exists > 0 {
				continue
			}

			if *dryRun {
				logger.WithFields(logrus.Fields{
					"business_id":  r.BusinessId,
					"warehouse_id": warehouseId,
					"product_id":   r.ProductId,
					"product_type": r.ProductType,
					"batch":        r.BatchNumber,
					"qty":          r.Qty.String(),
					"unit_value":   r.UnitValue.String(),
					"ref_type":     refType,
				}).Info("dry-run: would backfill opening stock to stock_histories")
				continue
			}

			stockDate, err := utils.ConvertToDate(biz.MigrationDate, biz.Timezone)
			if err != nil {
				panic(err)
			}

			err = db.Transaction(func(tx *gorm.DB) error {
				sh := models.StockHistory{
					BusinessId:        r.BusinessId,
					WarehouseId:       warehouseId,
					ProductId:         r.ProductId,
					ProductType:       r.ProductType,
					BatchNumber:       r.BatchNumber,
					StockDate:         stockDate,
					Qty:               r.Qty,
					Description:       "Opening Stock (backfill)",
					ReferenceType:     refType,
					ReferenceID:       refID,
					ReferenceDetailID: 0,
					IsOutgoing:        utils.NewFalse(),
					BaseUnitValue:     r.UnitValue,
				}
				if err := tx.Create(&sh).Error; err != nil {
					return err
				}

				if err := models.UpdateStockSummaryOpeningQty(tx, r.BusinessId, warehouseId, r.ProductId, string(r.ProductType), r.BatchNumber, r.Qty, stockDate); err != nil {
					return err
				}

				if *rebuild {
					if _, err := workflow.RebuildInventoryForItemWarehouseFromDate(tx, logger, r.BusinessId, warehouseId, r.ProductId, r.ProductType, r.BatchNumber, stockDate); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				panic(err)
			}
		}
	}

	fmt.Println("opening stock backfill completed")
}
