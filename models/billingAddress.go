package models

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

type BillingAddress struct {
	ID            int       `gorm:"primary_key" json:"id"`
	Attention     string    `gorm:"size:100" json:"attention"`
	Address       string    `gorm:"type:text" json:"address"`
	Country       string    `gorm:"size:100" json:"country"`
	City          string    `gorm:"size:100" json:"city"`
	StateId       int       `json:"state_id"`
	TownshipId    int       `json:"township_id"`
	Phone         string    `gorm:"size:20" json:"phone"`
	Mobile        string    `gorm:"size:20" json:"mobile"`
	Email         string    `gorm:"size:100" json:"email"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   int       `json:"reference_id"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewBillingAddress struct {
	Attention  string `json:"attention"`
	Address    string `json:"address"`
	Country    string `json:"country"`
	City       string `json:"city"`
	StateId    int    `json:"state_id"`
	TownshipId int    `json:"township_id"`
	Phone      string `json:"phone"`
	Mobile     string `json:"mobile"`
	Email      string `json:"email"`
}

// convert NewBillingAddresses to BillingAddresses
// using named returned values
func mapBillingAddressInput(input NewBillingAddress) (ret BillingAddress) {
	return BillingAddress{
		Attention:  input.Attention,
		Address:    input.Address,
		Country:    input.Country,
		City:       input.City,
		StateId:    input.StateId,
		TownshipId: input.TownshipId,
		Phone:      input.Phone,
		Mobile:     input.Mobile,
		Email:      input.Email,
	}
}

func upsertBillingAddress(tx *gorm.DB, ctx context.Context, input NewBillingAddress, referenceType string, referenceId int) (err error) {
	id, err := utils.GetPolymorphicId[BillingAddress](ctx, referenceType, referenceId)
	if err != nil {
		return
	}

	billingAddress := mapBillingAddressInput(input)
	if id == 0 {
		// insert new
		billingAddress.ReferenceID = referenceId
		billingAddress.ReferenceType = referenceType
		err = tx.WithContext(ctx).Create(&billingAddress).Error
	} else {
		// update
		err = tx.WithContext(ctx).Model(&BillingAddress{}).
			Session(&gorm.Session{SkipHooks: true}).
			Where("id = ?", id).Updates(map[string]interface{}{
			"Attention":  billingAddress.Attention,
			"Address":    billingAddress.Address,
			"Country":    billingAddress.Country,
			"City":       billingAddress.City,
			"StateId":    billingAddress.StateId,
			"TownshipId": billingAddress.TownshipId,
			"Phone":      billingAddress.Phone,
			"Mobile":     billingAddress.Mobile,
			"Email":      billingAddress.Email,
		}).Error
	}
	return
}

// func updateBillingAddress(tx *gorm.DB, input BillingAddress, referenceType string, referenceId int) error {
// 	if err := tx.Where("reference_type = ? AND reference_id = ?", referenceType, referenceId); err != nil {
// 		return err
// 	}

// }
