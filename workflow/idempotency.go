package workflow

import (
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/models"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrIdempotencyInProgress = errors.New("idempotency in progress")

func isDuplicateKeyErr(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}

// BeginIdempotency inserts STARTED. If SUCCEEDED exists, returns (true, nil) meaning "skip safely".
func BeginIdempotency(tx *gorm.DB, businessId, handlerName, messageId string) (skip bool, err error) {
	key := models.IdempotencyKey{
		BusinessId:  businessId,
		HandlerName: handlerName,
		MessageId:   messageId,
		Status:      models.IdempotencyStatusStarted,
	}
	if err := tx.Create(&key).Error; err == nil {
		return false, nil
	} else if !isDuplicateKeyErr(err) {
		return false, err
	}

	var existing models.IdempotencyKey
	if err := tx.Where("business_id = ? AND handler_name = ? AND message_id = ?", businessId, handlerName, messageId).
		First(&existing).Error; err != nil {
		return false, err
	}

	switch existing.Status {
	case models.IdempotencyStatusSucceeded:
		return true, nil
	case models.IdempotencyStatusStarted:
		// If another worker is currently processing, ask Pub/Sub to retry.
		// If it's stale, let it retry by reusing same row (set STARTED again).
		if time.Since(existing.UpdatedAt) < 5*time.Minute {
			return false, ErrIdempotencyInProgress
		}
		return false, tx.Model(&models.IdempotencyKey{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{"status": models.IdempotencyStatusStarted, "last_error": nil}).Error
	case models.IdempotencyStatusFailed:
		return false, tx.Model(&models.IdempotencyKey{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{"status": models.IdempotencyStatusStarted, "last_error": nil}).Error
	default:
		return false, tx.Model(&models.IdempotencyKey{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{"status": models.IdempotencyStatusStarted, "last_error": nil}).Error
	}
}

func MarkIdempotencySucceeded(tx *gorm.DB, businessId, handlerName, messageId string) error {
	return tx.Model(&models.IdempotencyKey{}).
		Where("business_id = ? AND handler_name = ? AND message_id = ?", businessId, handlerName, messageId).
		Updates(map[string]interface{}{"status": models.IdempotencyStatusSucceeded, "last_error": nil}).Error
}

func MarkIdempotencyFailed(tx *gorm.DB, businessId, handlerName, messageId string, err error) error {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return tx.Model(&models.IdempotencyKey{}).
		Where("business_id = ? AND handler_name = ? AND message_id = ?", businessId, handlerName, messageId).
		Updates(map[string]interface{}{"status": models.IdempotencyStatusFailed, "last_error": &msg}).Error
}

