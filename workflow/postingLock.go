package workflow

import (
	"fmt"

	"gorm.io/gorm"
)

// AcquireBusinessPostingLock serializes posting per business across instances using MySQL advisory locks.
// NOTE: GET_LOCK is connection-scoped, so this must be called on the same *gorm.DB that will do the posting transaction.
func AcquireBusinessPostingLock(tx *gorm.DB, businessId string) error {
	lockName := fmt.Sprintf("posting:%s", businessId)
	var ok int
	if err := tx.Raw("SELECT GET_LOCK(?, 30)", lockName).Scan(&ok).Error; err != nil {
		return err
	}
	if ok != 1 {
		return fmt.Errorf("could not acquire posting lock for business_id=%s", businessId)
	}
	return nil
}

func ReleaseBusinessPostingLock(tx *gorm.DB, businessId string) {
	lockName := fmt.Sprintf("posting:%s", businessId)
	var _ok int
	_ = tx.Raw("SELECT RELEASE_LOCK(?)", lockName).Scan(&_ok).Error
}

