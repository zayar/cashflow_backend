package utils

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// getGoogleClient initializes a Google Cloud Storage client
func getGoogleClient(ctx context.Context) (*storage.Client, error) {
	// Prefer ADC (Cloud Run service account / GOOGLE_APPLICATION_CREDENTIALS).
	// If you need to provide explicit JSON (e.g. locally), set GCS_CREDENTIALS_JSON.
	if credJSON := os.Getenv("GCS_CREDENTIALS_JSON"); strings.TrimSpace(credJSON) != "" {
		client, err := storage.NewClient(ctx, option.WithCredentialsJSON([]byte(credJSON)))
		if err != nil {
			return nil, err
		}
		return client, nil
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// GetGCSClient exposes the shared Google Cloud Storage client.
func GetGCSClient(ctx context.Context) (*storage.Client, error) {
	return getGoogleClient(ctx)
}

func SaveImageToGCS(ctx context.Context, objectName, imageData string) error {
	// Decode the base64 data
	decodedData, err := base64.StdEncoding.DecodeString(imageData)
	if err != nil {
		return err
	}
	bucketName := os.Getenv("GCS_BUCKET")
	if bucketName == "" {
		return errors.New("GCS_BUCKET is required")
	}

	// Get the Google Cloud Storage client
	client, err := getGoogleClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	if _, err := client.Bucket(bucketName).Attrs(ctx); err != nil {
		return fmt.Errorf("gcs bucket %q not found or not accessible: %v", bucketName, err)
	}

	// Upload the decoded image data to the specified object name in your GCS bucket
	contentType := "image/jpeg"
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = contentType
	wc.Metadata = map[string]string{
		"x-goog-acl": "public-read",
	}

	_, err = wc.Write(decodedData)
	if err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}

	return nil
}

func UploadFileToGCS(ctx context.Context, objectName string, fileContent io.Reader) error {
	// Get file content
	fileData, err := ioutil.ReadAll(fileContent)
	if err != nil {
		return fmt.Errorf("failed to read file content: %v", err)
	}

	mimeType := http.DetectContentType(fileData)

	// Manually set MIME type for .docx and .xlsx files
	if mimeType == "application/zip" {
		if strings.HasSuffix(objectName, ".docx") {
			mimeType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		} else if strings.HasSuffix(objectName, ".xlsx") {
			mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		}
	}

	// Define the allowed MIME types for each file type
	allowedMimeTypes := map[string]bool{
		"application/pdf":          true,
		"application/msword":       true,
		"application/vnd.ms-excel": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
		"image/jpeg": true,
		"image/png":  true,
		// Add more MIME types as needed
	}

	fmt.Println("mimeType", mimeType)
	fmt.Println("allowedMimeTypes", allowedMimeTypes)

	// Check if the MIME type is allowed
	if !allowedMimeTypes[mimeType] {
		return fmt.Errorf("unsupported file type: %s", mimeType)
	}

	// Get the Google Cloud Storage client
	client, err := getGoogleClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	bucketName := os.Getenv("GCS_BUCKET")
	if bucketName == "" {
		return errors.New("GCS_BUCKET is required")
	}

	if _, err := client.Bucket(bucketName).Attrs(ctx); err != nil {
		return fmt.Errorf("gcs bucket %q not found or not accessible: %v", bucketName, err)
	}

	// Upload the file to Google Cloud Storage
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = mimeType
	wc.Metadata = map[string]string{
		"x-goog-acl": "public-read",
	}

	if _, err := wc.Write(fileData); err != nil {
		return fmt.Errorf("failed to upload file to Google Cloud Storage: %v", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %v", err)
	}

	return nil
}

func UploadBytesToGCS(ctx context.Context, objectName string, data []byte, contentType string) error {
	client, err := getGoogleClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	bucketName := os.Getenv("GCS_BUCKET")
	if bucketName == "" {
		return errors.New("GCS_BUCKET is required")
	}
	if bucketName == "" {
		return errors.New("GCS_BUCKET is required")
	}

	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = contentType

	if _, err := wc.Write(data); err != nil {
		return fmt.Errorf("failed to upload bytes to Google Cloud Storage: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %v", err)
	}
	return nil
}

// DeleteImageFromGCS deletes an image from Google Cloud Storage
func DeleteImageFromGCS(ctx context.Context, objectName string) error {
	// Get the Google Cloud Storage client
	client, err := getGoogleClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	bucketName := os.Getenv("GCS_BUCKET")

	// Remove the specified object from your Bucket
	err = client.Bucket(bucketName).Object(objectName).Delete(ctx)
	if err != nil {
		// Check if the error is due to the object not existing
		if err == storage.ErrObjectNotExist {
			fmt.Println("Object does not exist:", objectName)
			return nil
		}
		return err
	}

	fmt.Println("Object deleted successfully:", objectName)
	return nil
}

// ObjectExists checks if an object exists in Google Cloud Storage
func ObjectExistsInGCS(ctx context.Context, objectName string) (bool, error) {
	// Get the Google Cloud Storage client
	client, err := getGoogleClient(ctx)
	if err != nil {
		return false, err
	}
	defer client.Close()

	bucketName := os.Getenv("GCS_BUCKET")

	// Attrs is used to check the existence of an object without downloading its content
	_, err = client.Bucket(bucketName).Object(objectName).Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil // Object does not exist
		}
		return false, err // Other error
	}

	return true, nil // Object exists
}

// CheckImageExistInCloud checks if an image exists on the internet
func CheckImageExistInGCS(imageURL string) error {
	if objectKey := ExtractObjectKeyFromURL(imageURL); objectKey != "" {
		ok, err := ObjectExistsInGCS(context.Background(), objectKey)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		return errors.New("image does not exist")
	}

	resp, err := http.Head(imageURL)
	if err != nil {
		return errors.New("invalid image url")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	return errors.New("image does not exist")
}
