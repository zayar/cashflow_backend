package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

type ReferenceDataUnion interface {
	// Common method for getting ID
	GetID() int
}

type Document struct {
	ID            int                `gorm:"primary_key" json:"id"`
	DocumentUrl   string             `json:"document_url"`
	ReferenceType string             `json:"reference_type"`
	ReferenceID   int                `json:"reference_id"`
	ReferenceData ReferenceDataUnion `gorm:"-" json:"referenceData"`
}

type NewDocument struct {
	HasId
	HasIsDeleted
	DocumentUrl string `json:"document_url"`
}

func mapNewDocuments(input []*NewDocument, referenceType string, referenceId int) ([]*Document, error) {

	// 2d return error
	var documents []*Document
	for _, i := range input {
		d, err := i.MapInput(referenceType, referenceId)
		if err != nil {
			return nil, err
		}
		documents = append(documents, d)
	}
	return documents, nil
}

// map for updating
// db.Model(m).Updates(...)
func (input NewDocument) Fillable() (map[string]interface{}, error) {
	// if err := utils.CheckImageExistInCloud(input.DocumentUrl); err != nil {
	if err := utils.CheckImageExistInGCS(input.DocumentUrl); err != nil {
		fmt.Println("Error checking document existence:", err)
		return nil, err
	}
	return map[string]interface{}{
		"DocumentUrl": input.DocumentUrl,
	}, nil
}

// for create
func (input NewDocument) MapInput(referenceType string, referenceId int) (*Document, error) {
	// if err := utils.CheckImageExistInCloud(input.DocumentUrl); err != nil {
	if err := utils.CheckImageExistInGCS(input.DocumentUrl); err != nil {
		fmt.Println("Error checking document existence:", err)
		return nil, err
	}
	return &Document{
		DocumentUrl:   input.DocumentUrl,
		ReferenceType: referenceType,
		ReferenceID:   referenceId,
	}, nil
}

func (d *Document) Store(tx *gorm.DB, ctx context.Context) error {
	return tx.WithContext(ctx).Create(&d).Error
}

func (d *Document) Delete(tx *gorm.DB, ctx context.Context) error {
	// delete actual file
	if err := tx.WithContext(ctx).Delete(&d).Error; err != nil {
		return err
	}
	// if err := utils.DeleteImageFromSpaces(extractObjectName(d.DocumentUrl)); err != nil {
	if err := utils.DeleteImageFromGCS(ctx, extractObjectName(d.DocumentUrl)); err != nil {
		return err
	}
	return nil
}

func (d *Document) Update(tx *gorm.DB, ctx context.Context, fillable map[string]interface{}) error {
	return tx.WithContext(ctx).Model(&d).Updates(fillable).Error
}

func GetDocument(ctx context.Context, id int) (*Document, error) {

	var result Document
	db := config.GetDB()
	if err := db.WithContext(ctx).First(&result, id).Error; err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	// Enforce tenant ownership (fail closed) unless explicitly bypassed for admin/internal ops.
	if skip, ok := utils.GetSkipTenantScopeFromContext(ctx); ok && skip {
		return &result, nil
	}
	if isAdmin, ok := utils.GetIsAdminFromContext(ctx); ok && isAdmin {
		return &result, nil
	}

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if result.ReferenceType == "" || result.ReferenceID <= 0 {
		return nil, errors.New("unauthorized")
	}

	// Validate the referenced record belongs to this business_id.
	tableByRefType := map[string]string{
		"customers":                     "customers",
		"suppliers":                     "suppliers",
		"purchase_orders":               "purchase_orders",
		"bills":                         "bills",
		"sales_orders":                  "sales_orders",
		"sales_invoices":                "sales_invoices",
		"customer_payments":             "customer_payments",
		"supplier_payments":             "supplier_payments",
		"supplier_credits":              "supplier_credits",
		"credit_notes":                  "credit_notes",
		"transfer_orders":               "transfer_orders",
		"inventory_adjustments":         "inventory_adjustments",
		"banking_transactions":          "banking_transactions",
		"account_transfer_transactions": "account_transfer_transactions",
		"expenses":                      "expenses",
		"journals":                      "journals",
		"products":                      "products",
		"product_groups":                "product_groups",
	}

	table, ok := tableByRefType[result.ReferenceType]
	if !ok || table == "" {
		// Unknown polymorphic type => deny rather than risk cross-tenant leakage.
		return nil, errors.New("unauthorized")
	}

	var count int64
	if err := db.WithContext(ctx).
		Table(table).
		Where("business_id = ? AND id = ?", businessId, result.ReferenceID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count <= 0 {
		return nil, errors.New("unauthorized")
	}

	return &result, nil
}

// document.updateMap() map[string]interface{}
// set/get referenceId, referenceType
// get/set id

// func upsertDocuments(ctx context.Context, tx *gorm.DB, input []NewDocument, referenceType string, referenceId int) ([]Document, error) {

// 	// documents := mapDocumentsInput(input, referenceType, referenceId)
// 	return UpsertPolymorphicAssociation(ctx, tx, input, referenceType, referenceId)
// }

// remove document
func RemoveFile(ctx context.Context, fullUrl string) (*UploadResponse, error) {

	// only remove image if not used in database
	var count int64
	db := config.GetDB()

	if err := db.Model(&Document{}).WithContext(ctx).Where("document_url = ?", fullUrl).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("cannot delete file associated with database")
	}

	// check if document exists
	objectName := extractObjectName(fullUrl)
	if objectName == "" {
		return nil, errors.New("invalid url")
	}
	// if ok, err := utils.ObjectExists(objectName); !ok || err != nil {
	if ok, err := utils.ObjectExistsInGCS(ctx, objectName); !ok || err != nil {
		return nil, errors.New("object does not exist")
	}

	// delete from cloud
	// if err := utils.DeleteImageFromSpaces(objectName); err != nil {
	if err := utils.DeleteImageFromGCS(ctx, objectName); err != nil {
		return nil, err
	}

	return &UploadResponse{
		ImageUrl: fullUrl,
	}, nil
}

func upsertDocuments(ctx context.Context, tx *gorm.DB, inputDocuments []*NewDocument, referenceType string, referenceId int) ([]*Document, error) {
	return UpsertPolymorphicAssociation(ctx, tx, inputDocuments, referenceType, referenceId)
}

func deleteDocuments(ctx context.Context, tx *gorm.DB, documents []*Document) error {
	for _, doc := range documents {
		if err := doc.Delete(tx, ctx); err != nil {
			return err
		}
	}
	return nil
}
