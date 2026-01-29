package main

import (
	"context"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/sirupsen/logrus"
)

func ensureBusinessContext(ctx context.Context, businessId string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if businessId == "" {
		return ctx
	}
	if _, ok := utils.GetBusinessIdFromContext(ctx); !ok {
		ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, businessId)
	}
	return ctx
}

func revertInvoiceToDraftOnDead(ctx context.Context, logger *logrus.Logger, msg config.PubSubMessage) {
	if msg.ReferenceType != string(models.AccountReferenceTypeInvoice) {
		return
	}
	if msg.ReferenceId <= 0 {
		return
	}

	ctx = ensureBusinessContext(ctx, msg.BusinessId)

	inv, err := models.GetSalesInvoice(ctx, msg.ReferenceId)
	if err != nil {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"field":          "OutboxDeadRevert",
				"business_id":    msg.BusinessId,
				"reference_type": msg.ReferenceType,
				"reference_id":   msg.ReferenceId,
			}).Warn("failed to load invoice for DEAD revert: " + err.Error())
		}
		return
	}
	if inv.CurrentStatus != models.SalesInvoiceStatusConfirmed {
		return
	}

	if _, err := models.UpdateStatusSalesInvoice(ctx, msg.ReferenceId, string(models.SalesInvoiceStatusDraft)); err != nil {
		if logger != nil {
			logger.WithFields(logrus.Fields{
				"field":          "OutboxDeadRevert",
				"business_id":    msg.BusinessId,
				"reference_type": msg.ReferenceType,
				"reference_id":   msg.ReferenceId,
			}).Warn("failed to revert invoice to Draft after DEAD posting: " + err.Error())
		}
		return
	}

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"field":          "OutboxDeadRevert",
			"business_id":    msg.BusinessId,
			"reference_type": msg.ReferenceType,
			"reference_id":   msg.ReferenceId,
		}).Info("reverted invoice to Draft after DEAD posting")
	}
}
