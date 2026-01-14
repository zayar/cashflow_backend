package workflow

import (
	"context"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RunPhase0ReconciliationChecks writes mismatch rows to reconciliation_reports.
// This is intended to be run on a schedule (nightly) or via an admin trigger.
func RunPhase0ReconciliationChecks(ctx context.Context, db *gorm.DB, logger *logrus.Logger, businessId string) error {
	// Delegate to the models-level implementation to avoid package cycles.
	_, err := models.RunPhase0ReconciliationChecks(ctx, businessId)
	if err != nil {
		return err
	}
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"field":       "ReconciliationChecks",
			"business_id": businessId,
		}).Info("phase0 reconciliation checks completed")
	}
	return nil
}
