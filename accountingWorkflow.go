package main

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	businessMutexMap = make(map[string]*sync.Mutex)
	globalMutex      = &sync.Mutex{}
)

func RunAccountingWorkflow() error {
	logger := config.GetLogger()
	ctx := context.Background()
	client, err := config.GetClient(ctx)
	if err != nil {
		return err
	}
	// topic := config.CreateTopicIfNotExists(client, businessId)
	topic, err := config.CreateTopicIfNotExists(client, os.Getenv("PUBSUB_TOPIC"))
	if err != nil {
		return err
	}
	// config.CreateSubscriptionIfNotExists(client, "c"+businessId, topic)
	sub, err := config.CreateSubscriptionIfNotExists(client, os.Getenv("PUBSUB_SUBSCRIPTION"), topic)
	if err != nil {
		return err
	}
	// Specify the number of concurrent processes
	sub.ReceiveSettings.MaxOutstandingMessages = 10

	// Create a callback function to handle messages.
	callback := func(ctx context.Context, msg *pubsub.Message) {
		m := config.PubSubMessage{}
		err := json.Unmarshal(msg.Data, &m)
		if err != nil {
			config.LogError(logger, "AccountingWorkflow.go", "RunAccountingWorkflow", "Unmarshaling pubsub message", msg.Data, err)
			return
		}

		// Get or create the mutex for the current BusinessId
		globalMutex.Lock()
		mutex, exists := businessMutexMap[m.BusinessId]
		if !exists {
			mutex = &sync.Mutex{}
			businessMutexMap[m.BusinessId] = mutex
		}
		globalMutex.Unlock()

		// Lock the specific business mutex
		mutex.Lock()
		defer mutex.Unlock()

		ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, m.BusinessId)
		ctx = context.WithValue(ctx, utils.ContextKeyUserId, 0)
		ctx = context.WithValue(ctx, utils.ContextKeyUserName, "System")
		if err := ProcessMessage(ctx, logger, m); err != nil {
			logger.WithFields(logrus.Fields{
				"field":          "AccountingWorkflow",
				"business_id":    m.BusinessId,
				"reference_type": m.ReferenceType,
				"reference_id":   m.ReferenceId,
				"message_id":     msg.ID,
			}).Error("pubsub processing failed: " + err.Error())
			msg.Nack()
			return
		}
		msg.Ack()
	}

	// Receive messages.
	go func() {
		err := sub.Receive(ctx, callback)

		if err != nil {
			config.LogError(logger, "AccountingWorkflow.go", "RunAccountingWorkflow", "Failed to receive messages", nil, err)
		}
	}()

	return nil
}

// func RunAccountingWorkflow(businessId string) error {
// 	js := config.GetJetstream()

// 	streamName := strings.ReplaceAll(businessId, "-", "")
// 	err := config.AddStream(js, streamName)
// 	if err != nil {
// 		return fmt.Errorf("add stream: %w", err)
// 	}
// 	consumerName := "c-" + streamName
// 	_, err = js.AddConsumer(streamName, &nats.ConsumerConfig{
// 		Durable:       consumerName,
// 		DeliverPolicy: nats.DeliverAllPolicy,
// 		AckPolicy:     nats.AckExplicitPolicy,
// 		AckWait:       30 * time.Minute,
// 		MaxDeliver:    10,
// 		MaxAckPending: -1,
// 	}, nats.Context(context.Background()))
// 	if err != nil && !errors.Is(err, nats.ErrConsumerNameAlreadyInUse) {
// 		return fmt.Errorf("add consumer: %w", err)
// 	}

// 	jetStreamSubscriber, err := js.PullSubscribe(
// 		config.SubjectPrefix+streamName,
// 		consumerName,
// 		nats.ManualAck(),
// 		nats.Bind(streamName, consumerName),
// 		nats.Context(context.Background()), // The context must be active until app is running.
// 	)
// 	if err != nil {
// 		return fmt.Errorf("pull subscribe: %w", err)
// 	}
// }

// func processSubcription(jetStreamSubscriber *nats.Subscription, businessId string) {
// 	logger := config.GetLogger()
// 	for {
// 		func() {
// 			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
// 			defer cancel()

// 			msgs, fetchErr := jetStreamSubscriber.Fetch(1, nats.Context(ctx))
// 			if fetchErr != nil {
// 				if errors.Is(fetchErr, context.DeadlineExceeded) {
// 					return
// 				}

// 				logger.WithFields(logrus.Fields{
// 					"field": "Accounting Workflow Fetch",
// 				}).Error(fetchErr.Error())
// 				return
// 			}

// 			for _, msg := range msgs {
// 				m := config.NatsMessage{}
// 				err := json.Unmarshal(msg.Data, &m)
// 				if err != nil {
// 					logger.WithFields(logrus.Fields{
// 						"field": "Accounting Workflow",
// 					}).Error(err.Error())
// 					continue
// 				}
// 				time.Sleep(7 * time.Second)
// 				db := config.GetDB()
// 				db.Transaction(func(tx *gorm.DB) error {
// 					if checkIfMessageAlreadyProcessed(m) {
// 						ackErr := msg.Ack()
// 						if ackErr != nil {
// 							logger.WithFields(logrus.Fields{
// 								"field": "Accounting Workflow Duplicate Acknowledge",
// 							}).Error(ackErr.Error())
// 							return ackErr
// 						}
// 					} else {
// 						workflowErr := processWorkflow(tx.WithContext(ctx), m, businessId)
// 						if workflowErr != nil {
// 							msg.Nak()
// 							logger.WithFields(logrus.Fields{
// 								"field": "Accounting Workflow Process",
// 							}).Error(workflowErr.Error())
// 							return workflowErr
// 						}
// 						ackErr := msg.Ack()
// 						if ackErr != nil {
// 							logger.WithFields(logrus.Fields{
// 								"field": "Accounting Workflow Acknowledge",
// 							}).Error(ackErr.Error())
// 							return ackErr
// 						}
// 						recordProcessedMessage(m)
// 					}
// 					return nil
// 				})
// 			}
// 		}()
// 	}
// }

func ProcessMessage(ctx context.Context, logger *logrus.Logger, m config.PubSubMessage) error {
	db := config.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		// Enforce strict per-business ordering across instances.
		if err := workflow.AcquireBusinessPostingLock(tx.WithContext(ctx), m.BusinessId); err != nil {
			return err
		}
		defer workflow.ReleaseBusinessPostingLock(tx.WithContext(ctx), m.BusinessId)

		// Worker-side posting gate: period locks must be enforced even if API validation was bypassed.
		if m.ReferenceType != "Reconcile" {
			if err := workflow.EnforcePostingGate(ctx, m); err != nil {
				now := time.Now().UTC()
				msg := err.Error()
				_ = tx.WithContext(ctx).Model(&models.PubSubMessageRecord{}).
					Where("id = ?", m.ID).
					Updates(map[string]interface{}{
						"is_processed":       true,
						"last_process_error": &msg,
						"processed_at":       &now,
					}).Error

				if logger != nil {
					logger.WithFields(logrus.Fields{
						"field":          "PostingGate",
						"business_id":    m.BusinessId,
						"reference_type": m.ReferenceType,
						"reference_id":   m.ReferenceId,
						"message_id":     m.ID,
					}).Warn("posting gate blocked message: " + err.Error())
				}
				// Ack/drop permanently (do not retry); message would otherwise loop forever.
				return nil
			}
		}

		if m.ReferenceType == "Reconcile" {
			// IMPORTANT: do not call tx.Commit()/tx.Rollback() inside db.Transaction.
			// Returning error triggers rollback; returning nil commits.
			if err := workflow.ProcessReconciliationWorkflow(tx.WithContext(ctx), logger, m); err != nil {
				return err
			}
			return nil
		} else {
			handlerName := m.ReferenceType
			messageId := strconv.Itoa(m.ID)

			skip, err := workflow.BeginIdempotency(tx.WithContext(ctx), m.BusinessId, handlerName, messageId)
			if err != nil {
				return err
			}
			if skip {
				return nil
			}

			if err := ProcessWorkflow(tx.WithContext(ctx), logger, m); err != nil {
				_ = workflow.MarkIdempotencyFailed(tx.WithContext(ctx), m.BusinessId, handlerName, messageId, err)
				return err
			}
			if err := workflow.MarkIdempotencySucceeded(tx.WithContext(ctx), m.BusinessId, handlerName, messageId); err != nil {
				return err
			}
			return nil
		}
	})
}

// NOTE: Redis TTL idempotency has been replaced by DB-backed IdempotencyKey (Phase 0).

// If any changes are made, change in reconciliationWorkflow.go too
func ProcessWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	switch msg.ReferenceType {
	case string(models.AccountReferenceTypeOpeningBalance):
		return workflow.ProcessOpeningBalanceWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeProductOpeningStock):
		return workflow.ProcessProductOpeningStockWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeProductGroupOpeningStock):
		return workflow.ProcessProductGroupOpeningStockWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeInventoryAdjustmentQuantity):
		return workflow.ProcessInventoryAdjustmentQuantityWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeInventoryAdjustmentValue):
		return workflow.ProcessInventoryAdjustmentValueWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeTransferOrder):
		return workflow.ProcessTransferOrderWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeAccountTransfer),
		string(models.AccountReferenceTypeAccountDeposit),
		string(models.AccountReferenceTypeOwnerContribution),
		string(models.AccountReferenceTypeOwnerDrawing),
		string(models.AccountReferenceTypeOtherIncome),
		string(models.AccountReferenceTypeAdvanceCustomerPayment),
		string(models.AccountReferenceTypeAdvanceSupplierPayment),
		string(models.AccountReferenceTypeSupplierCreditRefund),
		string(models.AccountReferenceTypeCreditNoteRefund),
		string(models.AccountReferenceTypeSupplierAdvanceRefund),
		string(models.AccountReferenceTypeCustomerAdvanceRefund):

		return workflow.ProcessBankingTransactionWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeBill):
		return workflow.ProcessBillWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeInvoice):
		return workflow.ProcessInvoiceWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeInvoiceWriteOff):
		return workflow.ProcessInvoiceWriteOffWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeCustomerOpeningBalance):
		return workflow.ProcessCustomerOpeningBalanceWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeCreditNote):
		return workflow.ProcessCreditNoteWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeCreditNoteApplied):
		return workflow.ProcessCreditNoteAppliedWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeExpense):
		return workflow.ProcessExpenseWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeJournal):
		return workflow.ProcessManualJournalWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeSupplierOpeningBalance):
		return workflow.ProcessSupplierOpeningBalanceWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeSupplierCredit):
		return workflow.ProcessSupplierCreditWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeSupplierCreditApplied):
		return workflow.ProcessSupplierCreditAppliedWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeSupplierPayment):
		return workflow.ProcessSupplierPaymentWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeCustomerPayment):
		return workflow.ProcessCustomerPaymentWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeSupplierAdvanceApplied):
		return workflow.ProcessSupplierAdvanceAppliedWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypeCustomerAdvanceApplied):
		return workflow.ProcessCustomerAdvanceAppliedWorkflow(tx, logger, msg)
	case string(models.AccountReferenceTypePosInvoicePayment):
		return workflow.ProcessPosInvoicePaymentWorkflow(tx, logger, msg)
	}
	return nil
}
