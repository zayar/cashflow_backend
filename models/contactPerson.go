package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type ContactPerson struct {
	ID            int       `gorm:"primary_key" json:"id"`
	Name          string    `gorm:"size:100" json:"name"`
	Email         string    `gorm:"size:100" json:"email"`
	Phone         string    `gorm:"size:20" json:"phone"`
	Mobile        string    `gorm:"size:20" json:"mobile"`
	Designation   string    `gorm:"size:100" json:"designation"`
	Department    string    `gorm:"size:100" json:"department"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   int       `json:"reference_id"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewContactPerson struct {
	HasId
	HasIsDeleted
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Mobile      string `json:"mobile"`
	Designation string `json:"designation"`
	Department  string `json:"department"`
}

func (cp *ContactPerson) Store(tx *gorm.DB, ctx context.Context) error {
	return tx.WithContext(ctx).Create(cp).Error
}

func (cp *ContactPerson) Delete(tx *gorm.DB, ctx context.Context) error {
	return tx.WithContext(ctx).Delete(cp).Error
}

func (cp *ContactPerson) Update(tx *gorm.DB, ctx context.Context, fillable map[string]interface{}) error {
	return tx.WithContext(ctx).Model(cp).Updates(fillable).Error
}

func (p NewContactPerson) Fillable() (map[string]interface{}, error) {
	return map[string]interface{}{
		"Name":        p.Name,
		"Email":       p.Email,
		"Phone":       p.Phone,
		"Mobile":      p.Mobile,
		"Designation": p.Designation,
		"Department":  p.Department,
	}, nil
}

func (i NewContactPerson) MapInput(referenceType string, referenceId int) (*ContactPerson, error) {
	return &ContactPerson{
		Name:          i.Name,
		Email:         i.Email,
		Phone:         i.Phone,
		Mobile:        i.Mobile,
		Designation:   i.Designation,
		Department:    i.Department,
		ReferenceType: referenceType,
		ReferenceID:   referenceId,
	}, nil
}

// convert NewContactPersons to ContactPersons
func mapNewContactPersons(input []*NewContactPerson, referenceType string, referenceId int) []*ContactPerson {
	var contactPeople []*ContactPerson
	// var names, emails, mobiles []string
	for _, i := range input {
		cp, _ := i.MapInput(referenceType, referenceId)
		contactPeople = append(contactPeople, cp)
	}

	return contactPeople
}

func upsertContactPersons(ctx context.Context, tx *gorm.DB, input []*NewContactPerson, referenceType string, referenceId int) ([]*ContactPerson, error) {
	return UpsertPolymorphicAssociation(ctx, tx, input, referenceType, referenceId)
}
