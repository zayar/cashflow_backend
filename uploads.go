package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/sirupsen/logrus"
)

type uploadContext struct {
	EntityType    string `json:"entityType"`
	EntityID      int    `json:"entityId"`
	Field         string `json:"field"`
	ReferenceType string `json:"referenceType"`
	ReferenceID   int    `json:"referenceId"`
}

type uploadSignRequest struct {
	FileName string        `json:"fileName"`
	MimeType string        `json:"mimeType"`
	Size     int64         `json:"size"`
	Context  uploadContext `json:"context"`
}

type uploadCompleteRequest struct {
	ObjectKey string        `json:"objectKey"`
	MimeType  string        `json:"mimeType"`
	Context   uploadContext `json:"context"`
}

type uploadSignResponse struct {
	UploadURL string            `json:"uploadUrl"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ObjectKey string            `json:"objectKey"`
	AccessURL string            `json:"accessUrl"`
	ExpiresAt string            `json:"expiresAt"`
}

type uploadCompleteResponse struct {
	ImageURL          string         `json:"imageUrl,omitempty"`
	ThumbnailURL      string         `json:"thumbnailUrl,omitempty"`
	ObjectKey         string         `json:"objectKey"`
	ThumbnailObjectKey string        `json:"thumbnailObjectKey,omitempty"`
	Document          *uploadDocument `json:"document,omitempty"`
}

type uploadDocument struct {
	ID          int    `json:"id"`
	DocumentURL string `json:"documentUrl"`
}

const maxUploadSizeBytes int64 = 5 * 1024 * 1024

var imageMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}

var attachmentMimeTypes = map[string]bool{
	"application/pdf": true,
	"application/msword": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"image/jpeg": true,
	"image/png":  true,
}

func signUploadHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := config.GetLogger()
		requestID := requestIDFromHeaders(c)

		user, err := getSessionUser(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req uploadSignRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		if req.FileName == "" || req.MimeType == "" || req.Size <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "fileName, mimeType and size are required"})
			return
		}
		if req.Size > maxUploadSizeBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file size exceeds 5MB limit"})
			return
		}

		entity := normalizeEntity(req.Context.EntityType, req.Context.ReferenceType)
		if entity == "" {
			entity = "uploads"
		}

		if isImageRequest(req) {
			if !imageMimeTypes[req.MimeType] {
				c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported image type"})
				return
			}
		} else if !attachmentMimeTypes[req.MimeType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type"})
			return
		}

		ext := strings.ToLower(filepath.Ext(req.FileName))
		if ext == "" {
			ext = extensionFromMimeType(req.MimeType)
		}
		if ext == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file extension is required"})
			return
		}

		objectKey := path.Join(user.BusinessId, entity, uuid.New().String()+ext)
		if utils.GetStorageProvider() != utils.StorageProviderGCS {
			c.JSON(http.StatusBadRequest, gin.H{"error": "storage provider not supported"})
			return
		}

		signed, err := utils.SignUpload(c.Request.Context(), objectKey, req.MimeType, 15*time.Minute)
		if err != nil {
			logUploadError(logger, err, utils.GetStorageProvider(), requestID)
			message := "failed to sign upload"
			if !strings.EqualFold(strings.TrimSpace(os.Getenv("GO_ENV")), "production") {
				message = fmt.Sprintf("failed to sign upload: %v", err)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": message})
			return
		}

		logger.WithFields(logrus.Fields{
			"tenant_id":  user.BusinessId,
			"mime_type":  req.MimeType,
			"size":       req.Size,
			"object_key": objectKey,
		}).Info("[upload.sign]")

		c.JSON(http.StatusOK, gin.H{
			"data": uploadSignResponse{
				UploadURL: signed.UploadURL,
				Method:    signed.Method,
				Headers:   signed.Headers,
				ObjectKey: signed.ObjectKey,
				AccessURL: signed.AccessURL,
				ExpiresAt: signed.ExpiresAt.UTC().Format(time.RFC3339),
			},
		})
	}
}

func completeUploadHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := config.GetLogger()
		requestID := requestIDFromHeaders(c)

		user, err := getSessionUser(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req uploadCompleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.ObjectKey == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "objectKey is required"})
			return
		}
		if !strings.HasPrefix(req.ObjectKey, user.BusinessId+"/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid object key"})
			return
		}

		ctx := c.Request.Context()
		ctx = utils.SetBusinessIdInContext(ctx, user.BusinessId)
		ctx = utils.SetUserIdInContext(ctx, user.ID)
		ctx = utils.SetUserNameInContext(ctx, user.Username)

		response := uploadCompleteResponse{
			ObjectKey: req.ObjectKey,
		}

		if isImageComplete(req) {
			thumbnailKey, err := createThumbnail(ctx, req.ObjectKey)
			if err != nil {
				logUploadError(logger, err, utils.GetStorageProvider(), requestID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate thumbnail"})
				return
			}
			response.ImageURL = utils.BuildObjectAccessURL(req.ObjectKey)
			response.ThumbnailURL = utils.BuildObjectAccessURL(thumbnailKey)
			response.ThumbnailObjectKey = thumbnailKey
		} else {
			if req.Context.ReferenceType == "" || req.Context.ReferenceID <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "referenceType and referenceId are required"})
				return
			}
			documentURL := utils.BuildObjectAccessURL(req.ObjectKey)
			doc, err := models.CreateAttachmentFromURL(ctx, documentURL, req.Context.ReferenceType, req.Context.ReferenceID)
			if err != nil {
				logUploadError(logger, err, utils.GetStorageProvider(), requestID)
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			response.Document = &uploadDocument{
				ID:          doc.ID,
				DocumentURL: doc.DocumentUrl,
			}
		}

		logger.WithFields(logrus.Fields{
			"object_key": req.ObjectKey,
			"status":     "completed",
		}).Info("[upload.complete]")

		c.JSON(http.StatusOK, gin.H{"data": response})
	}
}

func uploadObjectHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		objectKey := strings.TrimSpace(c.Query("key"))
		if objectKey == "" || strings.Contains(objectKey, "..") || strings.HasPrefix(objectKey, "/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid key"})
			return
		}

		if utils.GetStorageProvider() != utils.StorageProviderGCS {
			c.JSON(http.StatusBadRequest, gin.H{"error": "storage provider not supported"})
			return
		}

		client, err := utils.GetGCSClient(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "storage client error"})
			return
		}
		defer client.Close()

		bucket := strings.TrimSpace(os.Getenv("GCS_BUCKET"))
		if bucket == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "GCS_BUCKET is required"})
			return
		}
		obj := client.Bucket(bucket).Object(objectKey)
		attrs, err := obj.Attrs(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "object not found"})
			return
		}
		reader, err := obj.NewReader(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "object not found"})
			return
		}
		defer reader.Close()

		if attrs != nil && attrs.ContentType != "" {
			c.Writer.Header().Set("Content-Type", attrs.ContentType)
		}
		if attrs != nil && attrs.Size > 0 {
			c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", attrs.Size))
		}
		c.Status(http.StatusOK)
		_, _ = io.Copy(c.Writer, reader)
	}
}

func createThumbnail(ctx context.Context, objectKey string) (string, error) {
	client, err := utils.GetGCSClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	bucket := strings.TrimSpace(os.Getenv("GCS_BUCKET"))
	if bucket == "" {
		return "", errors.New("GCS_BUCKET is required")
	}

	reader, err := client.Bucket(bucket).Object(objectKey).NewReader(ctx)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxUploadSizeBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxUploadSizeBytes {
		return "", errors.New("file size exceeds 5MB limit")
	}

	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	thumbnail := imaging.Resize(img, 200, 0, imaging.Lanczos)

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, thumbnail, imaging.JPEG); err != nil {
		return "", err
	}

	thumbnailKey := thumbnailObjectKey(objectKey)
	if err := utils.UploadBytesToGCS(ctx, thumbnailKey, buf.Bytes(), "image/jpeg"); err != nil {
		return "", err
	}
	return thumbnailKey, nil
}

func thumbnailObjectKey(objectKey string) string {
	dir := path.Dir(objectKey)
	filename := path.Base(objectKey)
	return path.Join(dir, "thumbnails", filename)
}

func getSessionUser(ctx context.Context) (*models.User, error) {
	username, ok := utils.GetUsernameFromContext(ctx)
	if !ok || username == "" {
		return nil, errors.New("unauthorized")
	}

	var user models.User
	exists, err := config.GetRedisObject("User:"+username, &user)
	if err != nil {
		return nil, err
	}
	if !exists {
		db := config.GetDB()
		if db == nil {
			return nil, errors.New("db is nil")
		}
		if err := db.WithContext(ctx).Model(&models.User{}).Where("username = ?", username).Take(&user).Error; err != nil {
			return nil, errors.New("unauthorized")
		}
	}
	if user.BusinessId == "" {
		return nil, errors.New("unauthorized")
	}
	return &user, nil
}

func normalizeEntity(primary, fallback string) string {
	value := strings.TrimSpace(primary)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, " ", "_")
	value = sanitizeSegment(value)
	return value
}

func sanitizeSegment(input string) string {
	var out strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func extensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "application/pdf":
		return ".pdf"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	default:
		return ""
	}
}

func isImageRequest(req uploadSignRequest) bool {
	if strings.Contains(strings.ToLower(req.Context.EntityType), "image") {
		return true
	}
	if strings.Contains(strings.ToLower(req.Context.Field), "image") {
		return true
	}
	return strings.HasPrefix(req.MimeType, "image/")
}

func isImageComplete(req uploadCompleteRequest) bool {
	if strings.Contains(strings.ToLower(req.Context.EntityType), "image") {
		return true
	}
	if strings.Contains(strings.ToLower(req.Context.Field), "image") {
		return true
	}
	return false
}

func logUploadError(logger *logrus.Logger, err error, provider string, requestID string) {
	logger.WithFields(logrus.Fields{
		"error":      err.Error(),
		"provider":   provider,
		"request_id": requestID,
	}).Error("[upload.error]")
}

func requestIDFromHeaders(c *gin.Context) string {
	if id := strings.TrimSpace(c.GetHeader("X-Correlation-Id")); id != "" {
		return id
	}
	if id := strings.TrimSpace(c.GetHeader("X-Request-Id")); id != "" {
		return id
	}
	return fmt.Sprintf("upload-%d", time.Now().UnixNano())
}
