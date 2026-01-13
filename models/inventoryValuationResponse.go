package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type InventoryValuationResponse struct {
	OpeningStockOnHand decimal.Decimal             `json:"openingStockOnHand"`
	OpeningAssetValue  decimal.Decimal             `json:"openingAssetValue"`
	Details            []*InventoryValuationDetail `gorm:"-" json:"details,omitempty"`
	ClosingStockOnHand decimal.Decimal             `json:"closingStockOnHand"`
	ClosingAssetValue  decimal.Decimal             `json:"closingAssetValue"`
}

type InventoryValuationDetail struct {
	TransactionDate        time.Time       `json:"transactionDate"`
	TransactionDescription string          `json:"transactionDescription"`
	WarehouseName          *string         `json:"warehouse_name"`
	Qty                    decimal.Decimal `json:"qty"`
	UnitCost               decimal.Decimal `json:"unitCost"`
	StockOnHand            decimal.Decimal `json:"stockOnHand"`
	AssetValue             decimal.Decimal `json:"assetValue"`
}

func GetInventoryValuation(ctx context.Context, fromDate MyDateString, toDate MyDateString, productId int, productType ProductType, warehouseId int) (*InventoryValuationResponse, error) {

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
	if err := ValidateProductId(ctx, businessId, productId, productType); err != nil {
		return nil, err
	}

	var response InventoryValuationResponse
	db := config.GetDB()
	var openingSql string
	if warehouseId > 0 {
		openingSql = `
		SELECT
			sh.closing_asset_value as opening_asset_value,
			sh.closing_qty as opening_stock_on_hand
		from
			stock_histories sh
		where
			sh.business_id = @businessId
			AND sh.stock_date < @fromDate
			and product_id = @productId
			and product_type = @productType
			and warehouse_id = @warehouseId
		order by
			stock_date desc,
			cumulative_sequence DESC
			limit 1
				`
	} else {
		openingSql = `
-- get opening stockOnHand,assetValue from all warehouses
with LastStockHistories AS (
    SELECT
        closing_asset_value,
        closing_qty,
        (
            ROW_NUMBER() Over (
                PARTITION by business_id,
					warehouse_id,
                product_type,
                product_id
                order by
                    stock_date desc,
					cumulative_sequence DESC
            )
        ) as rn
    from
        stock_histories sh
    WHERE
		business_id = @businessId
        AND stock_date < @fromDate
        and product_id = @productId
        and product_type = @productType
)
SELECT
    sum(closing_asset_value) opening_asset_value,
    sum(closing_qty) opening_stock_on_hand
FROM
    LastStockHistories lsh
WHERE
    rn = 1
group by
    rn
		`

	}
	if err := db.WithContext(ctx).Raw(openingSql, map[string]interface{}{
		"businessId":  businessId,
		"productId":   productId,
		"productType": productType,
		"warehouseId": warehouseId,
		"fromDate":    fromDate,
	}).Scan(&response).Error; err != nil {
		return nil, err
	}

	// If there are no stock_histories rows strictly before fromDate, the opening balance
	// for the report should come from any opening-stock postings on fromDate itself.
	//
	// This prevents "Opening Stock = 0" reports for migrated businesses where the first
	// ever inventory posting is the migration/opening-stock entry dated on the report's start date.
	var priorCount int64
	countQuery := db.WithContext(ctx).Model(&StockHistory{}).
		Where("business_id = ? AND product_id = ? AND product_type = ?", businessId, productId, productType).
		Where("stock_date < ?", fromDate)
	if warehouseId > 0 {
		countQuery = countQuery.Where("warehouse_id = ?", warehouseId)
	}
	if err := countQuery.Count(&priorCount).Error; err != nil {
		return nil, err
	}

	excludeOpeningOnFromDate := false
	if priorCount == 0 {
		openingOnDateT := `
SELECT
	COALESCE(SUM(qty * base_unit_value), 0) AS opening_asset_value,
	COALESCE(SUM(qty), 0) AS opening_stock_on_hand
FROM
	stock_histories
WHERE
	business_id = @businessId
	AND product_id = @productId
	AND product_type = @productType
	-- stock_date is stored as a "date-only" value in business timezone in many flows.
	-- fromDate is normalized to UTC start-of-day; use DATE() to avoid timezone equality issues.
	AND DATE(stock_date) = DATE(@fromDate)
	AND reference_type IN ('POS', 'PGOS', 'PCOS')
	{{- if gt .warehouseId 0 }}
	AND warehouse_id = @warehouseId
	{{- end }}
`
		openingOnDateSQL, err := utils.ExecTemplate(openingOnDateT, map[string]interface{}{
			"warehouseId": warehouseId,
		})
		if err != nil {
			return nil, err
		}
		if err := db.WithContext(ctx).Raw(openingOnDateSQL, map[string]interface{}{
			"businessId":  businessId,
			"productId":   productId,
			"productType": productType,
			"warehouseId": warehouseId,
			"fromDate":    fromDate,
		}).Scan(&response).Error; err != nil {
			return nil, err
		}

		// Only exclude opening-stock postings from details if we actually used them as the opening balance.
		if !response.OpeningStockOnHand.IsZero() || !response.OpeningAssetValue.IsZero() {
			excludeOpeningOnFromDate = true
		}

		// Fallback: some datasets create opening stock rows during product creation but rely on async outbox
		// to post stock_histories. If that async posting hasn't happened yet, opening stock won't appear in
		// stock_histories. We can still compute opening stock from the opening_stocks table (scoped to business)
		// to avoid "Opening Stock = 0" and negative stock-on-hand in the report.
		if !excludeOpeningOnFromDate && response.OpeningStockOnHand.IsZero() && response.OpeningAssetValue.IsZero() {
			openingStocksFallbackSQLT := `
SELECT
	COALESCE(SUM(os.qty * os.unit_value), 0) AS opening_asset_value,
	COALESCE(SUM(os.qty), 0) AS opening_stock_on_hand
FROM opening_stocks os
{{- if eq .productType "S" }}
	INNER JOIN products p ON p.id = os.product_id AND p.business_id = @businessId
{{- else if eq .productType "V" }}
	INNER JOIN product_variants pv ON pv.id = os.product_id AND pv.business_id = @businessId
{{- else }}
	-- No fallback available for this productType in opening_stocks
	INNER JOIN products p ON 1 = 0
{{- end }}
WHERE
	os.product_id = @productId
	AND os.product_type = @productType
	{{- if gt .warehouseId 0 }}
	AND os.warehouse_id = @warehouseId
	{{- end }}
`
			openingStocksFallbackSQL, err := utils.ExecTemplate(openingStocksFallbackSQLT, map[string]interface{}{
				"warehouseId": warehouseId,
				"productType": string(productType),
			})
			if err != nil {
				return nil, err
			}
			if err := db.WithContext(ctx).Raw(openingStocksFallbackSQL, map[string]interface{}{
				"businessId":  businessId,
				"productId":   productId,
				"productType": productType,
				"warehouseId": warehouseId,
			}).Scan(&response).Error; err != nil {
				return nil, err
			}
		}
	}

	var details []*InventoryValuationDetail

	// IMPORTANT: always filter by business_id; product ids are not globally unique.
	// NOTE: All column references must be prefixed with stock_histories. to avoid
	// ambiguity when JOINing with warehouses (both tables have business_id).
	sqlT := `
SELECT
    stock_histories.id,
    -- warehouse_id,
	stock_histories.stock_date AS transaction_date,
	stock_histories.description AS transaction_description,
	warehouses.name AS warehouse_name,
    stock_histories.qty,
    stock_histories.base_unit_value unit_cost,
    @openingStockOnHand + SUM(stock_histories.qty) OVER (
        PARTITION BY
	{{- if gt .warehouseId 0}}
		stock_histories.warehouse_id,
	{{- end }}
		stock_histories.business_id,
        stock_histories.product_id,
        stock_histories.product_type,
        stock_histories.batch_number
        ORDER BY
            stock_histories.stock_date, stock_histories.cumulative_sequence ROWS BETWEEN UNBOUNDED PRECEDING
            AND CURRENT ROW
    ) AS stock_on_hand,
    @openingAssetValue + SUM(stock_histories.qty * stock_histories.base_unit_value) OVER (
        PARTITION BY
		{{- if gt .warehouseId 0}} stock_histories.warehouse_id, {{- end }}
		stock_histories.business_id,
        stock_histories.product_id,
        stock_histories.product_type,
        stock_histories.batch_number
        ORDER BY
            stock_histories.stock_date, stock_histories.cumulative_sequence ROWS BETWEEN UNBOUNDED PRECEDING
            AND CURRENT ROW
    ) AS asset_value
FROM
    stock_histories
	LEFT JOIN warehouses ON warehouses.id = stock_histories.warehouse_id
WHERE
{{- if gt .warehouseId 0}}
	stock_histories.warehouse_id = @warehouseId AND
{{- end }}
	stock_histories.business_id = @businessId
    AND stock_histories.product_id = @productId
    AND stock_histories.product_type = @productType
    AND stock_histories.stock_date BETWEEN @fromDate AND @toDate
{{- if .excludeOpeningOnFromDate }}
	AND NOT (
		DATE(stock_histories.stock_date) = DATE(@fromDate)
		AND stock_histories.reference_type IN ('POS', 'PGOS', 'PCOS')
	)
{{- end }}
ORDER BY stock_histories.stock_date, stock_histories.cumulative_sequence
`

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"warehouseId":              warehouseId,
		"excludeOpeningOnFromDate": excludeOpeningOnFromDate,
	})
	if err != nil {
		return nil, err
	}
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":         businessId,
		"productId":          productId,
		"productType":        productType,
		"warehouseId":        warehouseId,
		"fromDate":           fromDate,
		"toDate":             toDate,
		"openingStockOnHand": response.OpeningStockOnHand,
		"openingAssetValue":  response.OpeningAssetValue,
	}).Scan(&details).Error; err != nil {
		return nil, err
	}

	response.Details = details
	if len(details) > 0 {
		response.ClosingAssetValue = details[len(details)-1].AssetValue
		response.ClosingStockOnHand = details[len(details)-1].StockOnHand
	} else {
		response.ClosingAssetValue = response.OpeningAssetValue
		response.ClosingStockOnHand = response.OpeningStockOnHand
	}
	return &response, nil
}

type InventoryValuation struct {
	StockOnHand        decimal.Decimal `json:"stockOnHand"`
	AssetValue         decimal.Decimal `json:"assetValue"`
	UnitCost           decimal.Decimal `json:"unitCost"`
	ProductId          int             `json:"produtId"`
	ProductType        ProductType     `json:"produtType"`
	ProductUnitId      int             `json:"product_unit_id"`
	ProductDescription *string         `json:"product_description"`
}

func GetClosingInventoryValuation(ctx context.Context, currentDate MyDateString, warehouseId int, productId *int, productType *ProductType, batchNumber *string) ([]*InventoryValuation, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := currentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	if err := utils.ValidateResourceId[Warehouse](ctx, businessId, warehouseId); err != nil {
		return nil, errors.New("warehouse not found")
	}
	if productId != nil && productType != nil {
		if err := ValidateProductId(ctx, businessId, *productId, *productType); err != nil {
			return nil, err
		}
	}

	sqlT := `
WITH AllProductUnits AS (
    SELECT
        unit_id product_unit_id,
        id product_id,
        'S' as product_type,
		description product_description
    from
        products
	WHERE business_id = @businessId
    UNION
    (SELECT
        pv.unit_id product_unit_id,
        pv.id product_id,
        'V' as product_type,
		pg.description product_description
    from
        product_variants pv
	LEFT JOIN product_groups pg
		ON pv.product_group_id = pg.id
	WHERE pv.business_id = @businessId)
),
LastStockHistories AS (
    SELECT
        product_id,
        product_type,
        closing_qty stock_on_hand,
        closing_asset_value asset_value,
		base_unit_value unit_cost
    FROM
    (
        SELECT
            ROW_NUMBER() OVER (
                PARTITION BY
                product_id,
                product_type -- batch_number
                ORDER BY
                    cumulative_sequence DESC
            ) AS rn,
            warehouse_id,
            product_id,
            product_type,
            closing_qty,
            closing_asset_value,
			base_unit_value,
			stock_date
        FROM
            stock_histories
			WHERE business_id = @businessId AND
			stock_date <= @currentDate
                AND warehouse_id = @warehouseId
			{{- if .BatchNumber }}
				AND batch_number = @batchNumber
			{{- end }}
    )
    AS stock_histories_ranked
    WHERE
        rn = 1
)
SELECT
	sh.*,
	pu.product_unit_id,
	pu.product_description
FROM LastStockHistories sh
LEFT JOIN AllProductUnits pu
	ON sh.product_id = pu.product_id and sh.product_type = pu.product_type
	`

	db := config.GetDB()
	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"BatchNumber": utils.DereferencePtr(batchNumber),
	})
	if err != nil {
		return nil, err
	}

	var results []*InventoryValuation
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":  businessId,
		"currentDate": currentDate,
		"warehouseId": warehouseId,
		"batchNumber": batchNumber,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}

	// Fallback: some legacy datasets have stock_summaries populated but no stock_histories rows.
	// In that case, UI flows (invoice/bill/adjustment forms) would incorrectly show "0" stock.
	// We fall back to stock_summaries (current snapshot) ONLY when there are no stock_histories results.
	if len(results) == 0 {
		fallbackSQL := `
WITH AllProductUnits AS (
    SELECT
        unit_id product_unit_id,
        id product_id,
        'S' as product_type,
        description product_description
    FROM products
    WHERE business_id = @businessId
    UNION
    (SELECT
        pv.unit_id product_unit_id,
        pv.id product_id,
        'V' as product_type,
        pg.description product_description
    FROM product_variants pv
    LEFT JOIN product_groups pg
        ON pv.product_group_id = pg.id
    WHERE pv.business_id = @businessId)
)
SELECT
    ss.product_id,
    ss.product_type,
    ss.current_qty AS stock_on_hand,
    0 AS asset_value,
    0 AS unit_cost,
    pu.product_unit_id,
    pu.product_description
FROM stock_summaries ss
LEFT JOIN AllProductUnits pu
    ON ss.product_id = pu.product_id AND ss.product_type = pu.product_type
WHERE
    ss.business_id = @businessId
    AND ss.warehouse_id = @warehouseId
    AND ss.current_qty <> 0
`
		var fallback []*InventoryValuation
		if err := db.WithContext(ctx).Raw(fallbackSQL, map[string]interface{}{
			"businessId":  businessId,
			"warehouseId": warehouseId,
		}).Scan(&fallback).Error; err != nil {
			return nil, err
		}
		results = fallback
	}
	return results, nil
}
