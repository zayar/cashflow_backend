package pitixsync

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

func StatusHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)

		conn, err := getConnection(db, businessId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if conn == nil {
			modules := DefaultModules()
			c.JSON(http.StatusOK, StatusResponse{
				Connection: ConnectionResponse{
					Status: models.IntegrationStatusDisconnected,
				},
				Modules: modules,
			})
			return
		}

		modules := DecodeModules(conn.SettingsJSON)
		c.JSON(http.StatusOK, StatusResponse{
			Connection: ConnectionResponse{
				Status:     conn.Status,
				MerchantId: conn.StoreId,
				StoreName:  conn.StoreName,
			},
			LastSyncAt:        formatTime(conn.LastSyncAt),
			LastSuccessSyncAt: formatTime(conn.LastSuccessSyncAt),
			Modules:           modules,
		})
	}
}

func ConnectHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req ConnectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if strings.TrimSpace(req.StoreId) == "" || strings.TrimSpace(req.APIKey) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "storeId and apiKey are required"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)

		conn, err := getConnection(db, businessId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		modules := DefaultModules()
		now := time.Now()
		storeName := strings.TrimSpace(req.StoreName)
		if storeName == "" {
			storeName = req.StoreId
		}

		if conn == nil {
			conn = &models.IntegrationConnection{
				BusinessId:    businessId,
				Provider:      models.IntegrationProviderPitiX,
				Status:        models.IntegrationStatusConnected,
				AuthType:      "api_key",
				AuthSecretRef: req.APIKey,
				StoreId:       req.StoreId,
				StoreName:     storeName,
				SettingsJSON:  EncodeModules(modules),
				UpdatedAt:     now,
			}
			if err := db.Create(conn).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			update := map[string]interface{}{
				"status":          models.IntegrationStatusConnected,
				"auth_type":       "api_key",
				"auth_secret_ref": req.APIKey,
				"store_id":        req.StoreId,
				"store_name":      storeName,
				"updated_at":      now,
			}
			if len(conn.SettingsJSON) == 0 {
				update["settings_json"] = EncodeModules(modules)
			}
			if err := db.Model(conn).Updates(update).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func DisconnectHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)

		conn, err := getConnection(db, businessId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if conn == nil {
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}

		if err := db.Model(conn).Updates(map[string]interface{}{
			"status":          models.IntegrationStatusDisconnected,
			"auth_secret_ref": "",
			"updated_at":      time.Now(),
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func UpdateSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req UpdateSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)
		conn, err := getConnection(db, businessId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		modules := EncodeModules(req.Modules)
		if conn == nil {
			conn = &models.IntegrationConnection{
				BusinessId:   businessId,
				Provider:     models.IntegrationProviderPitiX,
				Status:       models.IntegrationStatusDisconnected,
				SettingsJSON: modules,
			}
			if err := db.Create(conn).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			if err := db.Model(conn).Updates(map[string]interface{}{
				"settings_json": modules,
				"updated_at":    time.Now(),
			}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func TriggerSyncHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req TriggerSyncRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)

		conn, err := getConnection(db, businessId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if conn == nil || conn.Status != models.IntegrationStatusConnected {
			c.JSON(http.StatusConflict, gin.H{"error": "pitix is not connected"})
			return
		}

		modules := req.Modules
		if isEmptyModules(modules) {
			modules = DecodeModules(conn.SettingsJSON)
		}

		run := models.IntegrationSyncRun{
			BusinessId:   businessId,
			ConnectionId: conn.ID,
			Provider:     models.IntegrationProviderPitiX,
			Status:       models.SyncRunStatusQueued,
			TriggeredBy:  models.SyncTriggeredManual,
			ModulesJSON:  EncodeModules(modules),
		}
		if err := db.Create(&run).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_ = PublishSyncRun(c.Request.Context(), run.ID, businessId, conn.ID)

		c.JSON(http.StatusOK, gin.H{"id": run.ID})
	}
}

func SyncHistoryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		limit := 20
		if v := strings.TrimSpace(c.Query("limit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)

		var runs []models.IntegrationSyncRun
		if err := db.Where("business_id = ? AND provider = ?", businessId, models.IntegrationProviderPitiX).
			Order("id desc").
			Limit(limit).
			Find(&runs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		items := make([]SyncRunResponse, 0, len(runs))
		for _, run := range runs {
			items = append(items, mapRunToResponse(run))
		}
		c.JSON(http.StatusOK, SyncHistoryResponse{Items: items})
	}
}

func SyncRunDetailHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run id"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)

		var run models.IntegrationSyncRun
		if err := db.Where("id = ? AND business_id = ?", id, businessId).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var errs []models.IntegrationSyncError
		if err := db.Where("sync_run_id = ?", run.ID).Order("id desc").Find(&errs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		resp := SyncRunDetailResponse{
			SyncRunResponse: mapRunToResponse(run),
			Errors:          mapErrors(errs),
		}
		c.JSON(http.StatusOK, resp)
	}
}

func RetrySyncRunHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		businessId, err := resolveBusinessID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run id"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), businessId)
		db := config.GetDB().WithContext(ctx)

		var run models.IntegrationSyncRun
		if err := db.Where("id = ? AND business_id = ?", id, businessId).Take(&run).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		newRun := models.IntegrationSyncRun{
			BusinessId:   businessId,
			ConnectionId: run.ConnectionId,
			Provider:     run.Provider,
			Status:       models.SyncRunStatusQueued,
			TriggeredBy:  models.SyncTriggeredRetry,
			ModulesJSON:  run.ModulesJSON,
			ParentRunId:  &run.ID,
		}
		if err := db.Create(&newRun).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_ = PublishSyncRun(c.Request.Context(), newRun.ID, businessId, run.ConnectionId)

		c.JSON(http.StatusOK, gin.H{"id": newRun.ID})
	}
}

func resolveBusinessID(c *gin.Context) (string, error) {
	username, ok := utils.GetUsernameFromContext(c.Request.Context())
	if !ok || strings.TrimSpace(username) == "" {
		return "", errors.New("unauthorized")
	}

	businessId := strings.TrimSpace(c.Query("business_id"))
	if businessId != "" {
		if err := authorizeInternalBusiness(c.Request.Context(), businessId); err != nil {
			return "", err
		}
		return businessId, nil
	}

	var user models.User
	exists, err := config.GetRedisObject("User:"+username, &user)
	if err != nil {
		return "", err
	}
	if !exists {
		db := config.GetDB()
		if db == nil {
			return "", errors.New("db is nil")
		}
		if err := db.WithContext(c.Request.Context()).
			Model(&models.User{}).
			Where("username = ?", username).
			Take(&user).Error; err != nil {
			return "", errors.New("unauthorized")
		}
	}
	businessId = strings.TrimSpace(user.BusinessId)
	if businessId == "" {
		return "", errors.New("business_id is required")
	}
	return businessId, nil
}

func authorizeInternalBusiness(ctx context.Context, businessId string) error {
	username, ok := utils.GetUsernameFromContext(ctx)
	if !ok || username == "" {
		return errors.New("unauthorized")
	}
	if businessId == "" {
		return errors.New("business_id is required")
	}

	var user models.User
	exists, err := config.GetRedisObject("User:"+username, &user)
	if err != nil {
		return err
	}
	if !exists {
		db := config.GetDB()
		if db == nil {
			return errors.New("db is nil")
		}
		if err := db.WithContext(ctx).Model(&models.User{}).Where("username = ?", username).Take(&user).Error; err != nil {
			return errors.New("unauthorized")
		}
	}

	if user.Role == models.UserRoleAdmin {
		return nil
	}
	if user.BusinessId != businessId {
		return errors.New("unauthorized")
	}
	return nil
}

func getConnection(db *gorm.DB, businessId string) (*models.IntegrationConnection, error) {
	var conn models.IntegrationConnection
	err := db.Where("business_id = ? AND provider = ?", businessId, models.IntegrationProviderPitiX).Take(&conn).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conn, nil
}

func formatTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func mapRunToResponse(run models.IntegrationSyncRun) SyncRunResponse {
	return SyncRunResponse{
		ID:            run.ID,
		Status:        run.Status,
		StartedAt:     formatTime(run.StartedAt),
		FinishedAt:    formatTime(run.FinishedAt),
		DurationMs:    run.DurationMs,
		RecordsSynced: run.RecordsSynced,
		ErrorCount:    run.ErrorCount,
		TriggeredBy:   run.TriggeredBy,
	}
}

func mapErrors(errorsList []models.IntegrationSyncError) []SyncErrorResponse {
	out := make([]SyncErrorResponse, 0, len(errorsList))
	for _, errItem := range errorsList {
		out = append(out, SyncErrorResponse{
			ID:         errItem.ID,
			EntityType: errItem.EntityType,
			ExternalId: errItem.ExternalId,
			Message:    errItem.Message,
			Retryable:  errItem.Retryable,
		})
	}
	return out
}

func isEmptyModules(mod SyncModules) bool {
	return !mod.Customers && !mod.Items && !mod.Invoices && !mod.Taxes && !mod.PaymentMethods && !mod.Warehouses
}
