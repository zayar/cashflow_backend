package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type supplierContactPersonReader struct {
	db *gorm.DB
}

func (r *supplierContactPersonReader) GetSupplierContactPersons(ctx context.Context, supplierIds []int) []*dataloader.Result[[]*models.ContactPerson] {
	var results []models.ContactPerson
	err := r.db.WithContext(ctx).Where("reference_type = 'suppliers' AND reference_id IN ?", supplierIds).Find(&results).Error
	if err != nil {
		return handleError[[]*models.ContactPerson](len(supplierIds), err)
	}

	return generateLoaderArrayResults(results, supplierIds)
	// // key => supplier id (int)
	// // value => array of billing contactPerson pointer []*contactPersons
	// resultMap := make(map[int][]*models.ContactPerson)
	// for _, result := range results {
	// 	resultMap[result.ReferenceID] = append(resultMap[result.ReferenceID], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.ContactPerson]
	// for _, id := range supplierIds {
	// 	contactPersons := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.ContactPerson]{Data: contactPersons})
	// }
	// return loaderResults
}

func GetSupplierContactPersons(ctx context.Context, supplierId int) ([]*models.ContactPerson, error) {
	loaders := For(ctx)
	return loaders.supplierContactPersonLoader.Load(ctx, supplierId)()
}
