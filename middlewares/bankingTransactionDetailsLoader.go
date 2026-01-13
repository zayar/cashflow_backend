package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type bankingTransactionDetailReader struct {
	db *gorm.DB
}

func (r *bankingTransactionDetailReader) GetBankingTransactionDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.BankingTransactionDetail] {
	var results []models.BankingTransactionDetail
	err := r.db.WithContext(ctx).Where("banking_transaction_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.BankingTransactionDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// key => customer id (int)
	// value => array of billing address pointer []*BillDetaile
	// resultMap := make(map[int][]*models.BillDetail)
	// for _, result := range results {
	// 	resultMap[result.BillId] = append(resultMap[result.BillId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.BillDetail]
	// for _, id := range Ids {
	// 	billDetails := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.BillDetail]{Data: billDetails})
	// }
	// return loaderResults
}

func GetBankingTransactionDetails(ctx context.Context, id int) ([]*models.BankingTransactionDetail, error) {
	loaders := For(ctx)
	return loaders.bankingTransactionDetailLoader.Load(ctx, id)()
}
