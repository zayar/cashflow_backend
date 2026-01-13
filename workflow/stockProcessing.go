package workflow

import (
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ProcessStockHistories processes a mixed list of stock histories by routing them
// to the correct processor based on IsOutgoing flag.
//
// IMPORTANT: run incoming first (ORDER BY stock_date, is_outgoing, id => incoming before outgoing).
func ProcessStockHistories(tx *gorm.DB, logger *logrus.Logger, stockHistories []*models.StockHistory) ([]int, error) {
	incoming := make([]*models.StockHistory, 0)
	outgoing := make([]*models.StockHistory, 0)

	for _, sh := range stockHistories {
		if sh == nil {
			continue
		}
		if sh.IsOutgoing != nil && *sh.IsOutgoing {
			outgoing = append(outgoing, sh)
		} else {
			incoming = append(incoming, sh)
		}
	}

	accountIds := make([]int, 0)
	if len(incoming) > 0 {
		ids, err := ProcessIncomingStocks(tx, logger, incoming)
		if err != nil {
			return nil, err
		}
		accountIds = utils.MergeIntSlices(accountIds, ids)
	}
	if len(outgoing) > 0 {
		ids, err := ProcessOutgoingStocks(tx, logger, outgoing)
		if err != nil {
			return nil, err
		}
		accountIds = utils.MergeIntSlices(accountIds, ids)
	}
	return accountIds, nil
}

