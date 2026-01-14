package graph

import (
	"context"
	"fmt"

	"github.com/mmdatafocus/books_backend/middlewares"
	"github.com/mmdatafocus/books_backend/models"
)

func GetAllProduct(ctx context.Context, productId int, productType models.ProductType) (*models.AllProduct, error) {
	var allProduct models.AllProduct
	var err error
	switch productType {
	case models.ProductTypeSingle:

		product, err := middlewares.GetProduct(ctx, productId)
		if err != nil {
			return nil, err
		}
		allProduct = models.AllProduct{
			Name:          product.Name,
			Sku:           &product.Sku,
			Barcode:       &product.Barcode,
			UnitId:        product.UnitId,
			SalesPrice:    product.SalesPrice,
			PurchasePrice: product.PurchasePrice,

			PurchaseAccountId: product.PurchaseAccountId,
			PurchaseTaxId:     product.PurchaseTaxId,
			PurchaseTaxType:   product.PurchaseTaxType,

			SalesAccountId: product.SalesAccountId,
			SalesTaxId:     product.SalesTaxId,
			SalesTaxType:   product.SalesTaxType,

			InventoryAccountId: product.InventoryAccountId,

			// IsActive:        *product.IsActive,
			// IsBatchTracking: *product.IsBatchTracking,
		}
		if product.IsActive != nil {
			allProduct.IsActive = *product.IsActive
		}
		if product.IsBatchTracking != nil {
			allProduct.IsBatchTracking = *product.IsBatchTracking
		}

	case models.ProductTypeVariant:

		productVariant, err := middlewares.GetProductVariant(ctx, productId)
		if err != nil {
			return nil, err
		}
		allProduct = models.AllProduct{
			Name:          productVariant.Name,
			Sku:           &productVariant.Sku,
			Barcode:       &productVariant.Barcode,
			UnitId:        productVariant.UnitId,
			SalesPrice:    productVariant.SalesPrice,
			PurchasePrice: productVariant.PurchasePrice,

			PurchaseAccountId: productVariant.PurchaseAccountId,
			PurchaseTaxId:     productVariant.PurchaseTaxId,
			PurchaseTaxType:   productVariant.PurchaseTaxType,

			SalesAccountId: productVariant.SalesAccountId,
			SalesTaxId:     productVariant.SalesTaxId,
			SalesTaxType:   productVariant.SalesTaxType,

			InventoryAccountId: productVariant.InventoryAccountId,

			// IsActive:        *product.IsActive,
			// IsBatchTracking: *product.IsBatchTracking,
		}
		if productVariant.IsActive != nil {
			allProduct.IsActive = *productVariant.IsActive
		}
		if productVariant.IsBatchTracking != nil {
			allProduct.IsBatchTracking = *productVariant.IsBatchTracking
		}
	}

	allProduct.ID = string(productType) + fmt.Sprint(productId)
	return &allProduct, err
}
