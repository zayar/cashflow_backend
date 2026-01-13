package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type InventoryValuationSummaryResponse struct {
	ProductID     int             `json:"productId"`
	ProductType   string          `json:"productType"`
	ProductName   string          `json:"productName"`
	Sku           string          `json:"sku"`
	ProductUnitId int             `json:"productUnitIt"`
	StockOnHand   decimal.Decimal `json:"stockOnHand"`
	AssetValue    decimal.Decimal `json:"assetValue"`
}

func GetInventoryValuationSummaryReport(ctx context.Context, currentDate models.MyDateString, warehouseId int) ([]*InventoryValuationSummaryResponse, error) {
	sqlT := `
WITH QtySummary AS (
    SELECT
        product_id,
        product_type,
        SUM(
            received_qty
            + adjusted_qty_in
            + transfer_qty_in
            - sale_qty
            - ABS(adjusted_qty_out)
            - ABS(transfer_qty_out)
        ) AS stock_on_hand
    FROM
        stock_summary_daily_balances
    WHERE
        transaction_date <= @currentDate
        AND business_id = @businessId
        {{- if not .AllWarehouse }}
            AND warehouse_id = @warehouseId
        {{- end }}
    GROUP BY
        product_id,
        product_type
),
LastStockHistories AS (

    SELECT
        product_id,
        product_type,
    {{- if .AllWarehouse }}
        SUM(closing_qty) as closing_qty,
        SUM(closing_asset_value) as closing_asset_value
    {{- else }}
        closing_qty,
        closing_asset_value
    {{- end }}
    FROM
    (
        SELECT
            ROW_NUMBER() OVER (
                PARTITION BY
                {{- if .AllWarehouse }}
                    business_id,
                    warehouse_id,
                {{- else }}
                    business_id,
                {{- end }}
                product_id,
                product_type -- batch_number
                ORDER BY
                    cumulative_sequence DESC
            ) AS rn,
            business_id,
            warehouse_id,
            product_id,
            product_type,
            closing_qty,
            closing_asset_value
        FROM
            stock_histories
            where business_id = @businessId
            AND stock_date <= @currentDate
            {{- if not .AllWarehouse }}
                AND warehouse_id = @warehouseId
            {{- end }}
    )
    AS stock_histories_ranked

    WHERE
        rn = 1
    {{- if .AllWarehouse }}
        GROUP BY
        product_id,
        product_type
    {{- end }}
),
AllProducts AS (
    SELECT
        id AS product_id,
        name AS product_name,
        unit_id AS product_unit_id,
        sku,
        'S' AS product_type
    FROM
        products
    WHERE
        business_id = @businessId
    UNION
    SELECT
        id AS product_id,
        name AS product_name,
        unit_id AS product_unit_id,
        sku,
        'V' AS product_type
    FROM
        product_variants
    WHERE
        business_id = @businessId
)
SELECT
    q.product_id,
    q.product_type,
    q.stock_on_hand as stock_on_hand,
    COALESCE(h.closing_asset_value, 0) as asset_value,
    p.product_name,
    p.product_unit_id,
    p.sku
FROM
    AllProducts p
    INNER JOIN QtySummary q on p.product_id = q.product_id
    AND p.product_type = q.product_type
    LEFT JOIN LastStockHistories h on p.product_id = h.product_id
    AND p.product_type = h.product_type
    ORDER BY p.product_name;
`

	// 	sqlOneWarehouse := `
	// WITH LastStockHistories AS (
	//     SELECT
	//         product_id,
	//         product_type,
	//         closing_qty,
	//         closing_asset_value,
	//         ROW_NUMBER() OVER (
	//             PARTITION BY
	//             product_id,
	//             product_type
	//             ORDER BY
	//                 stock_date DESC,
	//                 id DESC
	//         ) AS rn
	//     FROM
	//         stock_histories
	//     WHERE
	//     stock_date <= @currentDate
	//         AND warehouse_id = @warehouseId
	// ),
	// AllProducts AS (
	//     SELECT
	//         id AS product_id,
	//         name AS product_name,
	//         unit_id AS product_unit_id,
	//         sku AS product_sku,
	//         'S' AS product_type
	//     FROM
	//         products
	//         WHERE business_id = @businessId
	//     UNION
	//     SELECT
	//         id AS product_id,
	//         name AS product_name,
	//         unit_id AS product_unit_id,
	//         sku AS product_sku,
	//         'V' AS product_type
	//     FROM
	//         product_variants
	//         WHERE business_id = @businessId
	// )
	// SELECT
	//     h.closing_qty stock_on_hand,
	//     h.closing_asset_value asset_value,
	//     h.product_id,
	//     p.product_name,
	//     p.product_type,
	//     p.product_sku sku,
	//     p.product_unit_id
	// FROM
	//     AllProducts p
	// LEFT JOIN LastStockHistories h ON h.product_id = p.product_id
	//     AND h.product_type = p.product_type
	// WHERE
	//     h.rn = 1
	// ORDER BY p.product_name;
	// `
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := currentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"AllWarehouse": warehouseId == 0,
	})
	if err != nil {
		return nil, err
	}
	var results []*InventoryValuationSummaryResponse
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":  businessId,
		"currentDate": currentDate,
		"warehouseId": warehouseId,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
