package models

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type NewAttachment struct {
	Upload        graphql.Upload `json:"upload"`
	ReferenceType string         `json:"referenceType"`
	ReferenceID   int            `json:"referenceId"`
}

// document's reference type
func validateReferenceType(ctx context.Context, businessId string, referenceType string, referenceId int) error {
	db := config.GetDB()
	validReferenceTypes := map[string]bool{
		"banking_transactions":  true,
		"bills":                 true,
		"credit_notes":          true,
		"customers":             true,
		"customer_payments":     true,
		"expenses":              true,
		"inventory_adjustments": true,
		"journals":              true,
		"purchase_orders":       true,
		"sales_invoices":        true,
		"sales_orders":          true,
		"suppliers":             true,
		"supplier_credits":      true,
		"supplier_payments":     true,
		"transfer_orders":       true,
	}
	if ok := validReferenceTypes[referenceType]; !ok {
		return errors.New("invalid reference type")
	}

	// check if it exists
	var count int64
	if err := db.WithContext(ctx).Table(referenceType).Where("business_id = ? AND id = ?", businessId, referenceId).Count(&count).Error; err != nil {
		return err
	}
	if count <= 0 {
		return utils.ErrorRecordNotFound
	}

	return nil
}

func CreateAttachment(ctx context.Context, file graphql.Upload, referenceType string, referenceId int) (*Document, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate if the reference exists
	if err := validateReferenceType(ctx, businessId, referenceType, referenceId); err != nil {
		return nil, err
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		return nil, errors.New("file has no extension")
	}
	objectURL := path.Join(businessId, referenceType, uuid.New().String()+ext)

	// Upload file to storage provider
	// err := utils.UploadFileToSpace(objectName, file.File)
	err := utils.UploadFileToGCS(ctx, objectURL, file.File)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to storage provider: %v", err)
	}

	// fileURL := "https://" + os.Getenv("GCS_URL") + "/" + os.Getenv("GCS_BUCKET") + "/" + objectURL
	fileURL := getCloudURL(objectURL)
	var result Document = Document{
		DocumentUrl:   fileURL,
		ReferenceType: referenceType,
		ReferenceID:   referenceId,
	}
	db := config.GetDB()
	if err := db.WithContext(ctx).Create(&result).Error; err != nil {
		return nil, err
	}

	return &result, nil
}

func CreateAttachmentFromURL(ctx context.Context, documentURL string, referenceType string, referenceId int) (*Document, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := validateReferenceType(ctx, businessId, referenceType, referenceId); err != nil {
		return nil, err
	}

	if err := utils.CheckImageExistInGCS(documentURL); err != nil {
		return nil, err
	}

	var result Document = Document{
		DocumentUrl:   documentURL,
		ReferenceType: referenceType,
		ReferenceID:   referenceId,
	}
	db := config.GetDB()
	if err := db.WithContext(ctx).Create(&result).Error; err != nil {
		return nil, err
	}

	return &result, nil
}

func DeleteAttachment(ctx context.Context, id int) (*Document, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	var result Document
	if err := db.WithContext(ctx).First(&result, id).Error; err != nil {
		return nil, utils.ErrorRecordNotFound
	}
	if err := result.Delete(db, ctx); err != nil {
		return nil, err
	}
	return &result, nil
}
