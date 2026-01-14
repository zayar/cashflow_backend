package models

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

type ShippingAddress struct {
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

type NewShippingAddress struct {
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

// convert NewShippingAddresses to ShippingAddresses
func mapShippingAddressInput(input NewShippingAddress) (ret ShippingAddress) {
	return ShippingAddress{
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

func upsertShippingAddress(tx *gorm.DB, ctx context.Context, input NewShippingAddress, referenceType string, referenceId int) (err error) {
	id, err := utils.GetPolymorphicId[ShippingAddress](ctx, referenceType, referenceId)
	if err != nil {
		return
	}

	shippingAddress := mapShippingAddressInput(input)
	if id == 0 {
		// insert new
		shippingAddress.ReferenceID = referenceId
		shippingAddress.ReferenceType = referenceType
		err = tx.WithContext(ctx).Create(&shippingAddress).Error
	} else {
		// update
		err = tx.WithContext(ctx).Model(&ShippingAddress{}).
			Session(&gorm.Session{SkipHooks: true}).
			Where("id = ?", id).Updates(map[string]interface{}{
			"Attention":  shippingAddress.Attention,
			"Address":    shippingAddress.Address,
			"Country":    shippingAddress.Country,
			"City":       shippingAddress.City,
			"StateId":    shippingAddress.StateId,
			"TownshipId": shippingAddress.TownshipId,
			"Phone":      shippingAddress.Phone,
			"Mobile":     shippingAddress.Mobile,
			"Email":      shippingAddress.Email,
		}).Error
	}
	return
}
