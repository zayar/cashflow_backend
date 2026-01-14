package models

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/disintegration/imaging"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

type Image struct {
	ID            int    `gorm:"primary_key" json:"id"`
	ImageUrl      string `json:"image_url"`
	ThumbnailUrl  string `json:"thumbnail_url"`
	ReferenceType string `json:"reference_type"`
	ReferenceID   int    `json:"reference_id"`
}

type NewImage struct {
	HasId
	HasIsDeleted
	ImageUrl     string `json:"image_url"`
	ThumbnailUrl string `json:"thumbnail_url"`
}

type UploadResponse struct {
	ImageUrl     string `json:"image_url"`
	ThumbnailUrl string `json:"thumbnail_url"`
}

func mapNewImages(imageInput []*NewImage, referenceType string, referenceId int) ([]*Image, error) {

	var images []*Image

	for _, input := range imageInput {
		image, err := input.MapInput(referenceType, referenceId)
		if err != nil {
			return nil, err
		}

		images = append(images, image)
	}
	return images, nil
}

func UploadSingleImage(ctx context.Context, file graphql.Upload) (*UploadResponse, error) {

	originalCloudURL, thumbnailCloudURL, err := UploadImage(ctx, file)
	if err != nil {
		return nil, err
	}

	response := &UploadResponse{
		ImageUrl:     originalCloudURL,
		ThumbnailUrl: thumbnailCloudURL,
	}

	// Return the response and nil error on success
	return response, nil
}

// delete image,

// remove single image, including thumbnail
func RemoveImage(ctx context.Context, fullUrl string) (*UploadResponse, error) {

	// only remove image if not used in database
	var count int64
	db := config.GetDB()

	if err := db.Model(&Image{}).WithContext(ctx).Where("image_url = ?", fullUrl).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("cannot delete image associated with database")
	}

	// check if image exists
	objectName := extractObjectName(fullUrl)
	if objectName == "" {
		return nil, errors.New("invalid url")
	}
	// if ok, err := utils.ObjectExists(objectName); !ok || err != nil {
	if ok, err := utils.ObjectExistsInGCS(ctx, objectName); !ok || err != nil {
		return nil, errors.New("object does not exist")
	}

	// remove image + thumbnail from cloud
	// remove image
	// if err := utils.DeleteImageFromSpaces(objectName); err != nil {
	if err := utils.DeleteImageFromGCS(ctx, objectName); err != nil {
		return nil, err
	}
	storagePath := strings.Split(objectName, "/")[0]
	filename := strings.Split(objectName, "/")[1]
	// remove thumbnail
	thumbnailObjectName := filepath.Join(storagePath, "thumbnails", filename)
	// if err := utils.DeleteImageFromSpaces(thumbnailObjectName); err != nil {
	if err := utils.DeleteImageFromGCS(ctx, thumbnailObjectName); err != nil {
		return nil, err
	}

	return &UploadResponse{
		ImageUrl:     getCloudURL(objectName),
		ThumbnailUrl: getCloudURL(thumbnailObjectName),
	}, nil
}

func UploadMultipleImages(ctx context.Context, files []*graphql.Upload) ([]*UploadResponse, error) {
	var responseData []*UploadResponse

	for _, file := range files {
		originalCloudURL, thumbnailCloudURL, err := UploadImage(ctx, *file)

		if err != nil {
			return nil, err
		}

		uploadResponse := UploadResponse{
			ImageUrl:     originalCloudURL,
			ThumbnailUrl: thumbnailCloudURL,
		}

		responseData = append(responseData, &uploadResponse)
	}

	return responseData, nil
}

func UploadImage(ctx context.Context, file graphql.Upload) (string, string, error) {
	businessId, _ := utils.GetBusinessIdFromContext(ctx)

	if file.File == nil {
		return "", "", errors.New("nil file provided")
	}

	// Read the uploaded file
	data, err := io.ReadAll(file.File)
	if err != nil {
		return "", "", err
	}

	// Encode the file data to base64
	imageData := base64.StdEncoding.EncodeToString(data)

	// Extract the file extension
	ext := filepath.Ext(file.Filename)
	if ext == "" {
		return "", "", errors.New("file has no extension")
	}
	storagePath := "products/"
	uniqueFilename := businessId + " " + utils.GenerateUniqueFilename() + ext
	originalImageObjectURL := filepath.Join(storagePath, uniqueFilename)
	thumbnailImageObjectURL := filepath.Join(storagePath, "thumbnails", uniqueFilename)

	// Save the original image to Minio
	// err = utils.SaveImageToSpaces(originalImageObjectURL, imageData)
	err = utils.SaveImageToGCS(ctx, originalImageObjectURL, imageData)
	if err != nil {
		return "", "", err
	}

	// Generate and save the thumbnail
	thumbnailData, err := generateThumbnail(data)
	if err != nil {
		return "", "", err
	}

	// Encode the thumbnail data to base64
	thumbnailImageData := base64.StdEncoding.EncodeToString(thumbnailData)

	// Save the thumbnail to Minio
	// err = utils.SaveImageToSpaces(thumbnailImageObjectURL, thumbnailImageData)
	err = utils.SaveImageToGCS(ctx, thumbnailImageObjectURL, thumbnailImageData)
	if err != nil {
		return "", "", err
	}

	// Construct URLs for both original and thumbnail images
	originalCloudURL := getCloudURL(originalImageObjectURL)
	thumbnailCloudURL := getCloudURL(thumbnailImageObjectURL)

	return originalCloudURL, thumbnailCloudURL, nil
}

func getCloudURL(objectName string) string {
	// return "https://" + os.Getenv("SP_BUCKET") + "." + os.Getenv("SP_URL") + "/" + objectName
	return "https://" + os.Getenv("GCS_URL") + "/" + os.Getenv("GCS_BUCKET") + "/" + objectName
}

func extractObjectName(cloudUrl string) string {
	// baseUrl := "https://" + os.Getenv("SP_BUCKET") + "." + os.Getenv("SP_URL") + "/"
	baseUrl := "https://" + os.Getenv("GCS_URL") + "/" + os.Getenv("GCS_BUCKET") + "/"
	objectName, found := strings.CutPrefix(cloudUrl, baseUrl)
	if !found {
		return ""
	}
	return objectName
}

func UploadFile(ctx context.Context, file graphql.Upload) (*UploadResponse, error) {

	objectName := "documents/" + file.Filename

	// Upload file to DigitalOcean Space
	// err := utils.UploadFileToSpace(objectName, file.File)
	err := utils.UploadFileToGCS(ctx, objectName, file.File)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file to DigitalOcean Space: %v", err)
	}

	// fileURL := "https://" + os.Getenv("SP_BUCKET") + "." + os.Getenv("SP_URL") + "/" + objectName
	fileURL := "https://" + os.Getenv("GCS_URL") + "/" + os.Getenv("GCS_BUCKET") + "/" + objectName

	response := &UploadResponse{
		ImageUrl:     fileURL,
		ThumbnailUrl: "",
	}

	return response, nil
}

func generateThumbnail(originalData []byte) ([]byte, error) {
	// Decode the original image
	img, err := imaging.Decode(bytes.NewReader(originalData))
	if err != nil {
		return nil, err
	}

	// Resize the image to create a thumbnail
	thumbnail := imaging.Resize(img, 200, 0, imaging.Lanczos)

	// Encode the thumbnail to JPEG format
	var thumbnailBuffer bytes.Buffer
	err = imaging.Encode(&thumbnailBuffer, thumbnail, imaging.JPEG)
	if err != nil {
		return nil, err
	}

	return thumbnailBuffer.Bytes(), nil
}

func (img *Image) Store(tx *gorm.DB, ctx context.Context) error {
	if err := tx.WithContext(ctx).Create(&img).Error; err != nil {
		return err
	}
	return nil

}

func (img *Image) Update(tx *gorm.DB, ctx context.Context, data map[string]interface{}) error {
	// update existing image
	if err := tx.WithContext(ctx).Model(&img).Updates(data).Error; err != nil {
		return err
	}
	return nil
}

// expected img is loaded from db
func (img *Image) Delete(tx *gorm.DB, ctx context.Context) error {

	if err := tx.WithContext(ctx).Delete(&img).Error; err != nil {
		return err
	}
	// if err := utils.DeleteImageFromSpaces(extractObjectName(img.ImageUrl)); err != nil {
	if err := utils.DeleteImageFromGCS(ctx, extractObjectName(img.ImageUrl)); err != nil {
		return err
	}
	// if err := utils.DeleteImageFromSpaces(extractObjectName(img.ThumbnailUrl)); err != nil {
	if err := utils.DeleteImageFromGCS(ctx, extractObjectName(img.ThumbnailUrl)); err != nil {
		return err
	}
	return nil
}

// map newImage to Image, for db.Create(&image)
func (input NewImage) MapInput(referenceType string, referenceId int) (*Image, error) {
	// if err := utils.CheckImageExistInCloud(input.ImageUrl); err != nil {
	if err := utils.CheckImageExistInGCS(input.ImageUrl); err != nil {
		fmt.Println("Error checking image existence:", err)
		return nil, err
	}
	// if err := utils.CheckImageExistInCloud(input.ThumbnailUrl); err != nil {
	if err := utils.CheckImageExistInGCS(input.ThumbnailUrl); err != nil {
		fmt.Println("Error checking thumnail existence:", err)
		return nil, err
	}
	return &Image{
		ReferenceType: referenceType,
		ReferenceID:   referenceId,
		ImageUrl:      input.ImageUrl,
		ThumbnailUrl:  input.ThumbnailUrl,
	}, nil
}

func (input NewImage) Fillable() (map[string]interface{}, error) {
	// if err := utils.CheckImageExistInCloud(input.ImageUrl); err != nil {
	if err := utils.CheckImageExistInGCS(input.ImageUrl); err != nil {
		fmt.Println("Error checking image existence:", err)
		return nil, err
	}
	// if err := utils.CheckImageExistInCloud(input.ThumbnailUrl); err != nil {
	if err := utils.CheckImageExistInGCS(input.ThumbnailUrl); err != nil {
		fmt.Println("Error checking thumnail existence:", err)
		return nil, err
	}
	return map[string]interface{}{
		"ImageUrl":     input.ImageUrl,
		"ThumbnailUrl": input.ThumbnailUrl,
	}, nil
}

func UpsertImages(ctx context.Context, tx *gorm.DB, inputImages []*NewImage, referenceType string, referenceId int) ([]*Image, error) {
	return UpsertPolymorphicAssociation(ctx, tx, inputImages, referenceType, referenceId)
}
