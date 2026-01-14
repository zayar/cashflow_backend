package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/bsm/redislock"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/directives"
	"github.com/mmdatafocus/books_backend/graph"
	"github.com/mmdatafocus/books_backend/middlewares"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

const defaultPort = "8080"

var tracer = otel.Tracer("mkitchen-distribution")

type Cache struct {
	client redis.UniversalClient
	ttl    time.Duration
}

// Define a struct to represent the rate limiter.
type RateLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
}

type PubSubMessage struct {
	Message struct {
		Data []byte `json:"data,omitempty"`
		ID   string `json:"id"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

const apqPrefix = "apq:"

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getRedisClient(redisAddress string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddress,
	})
	return client
}

func NewCache(redisAddress string, ttl time.Duration) (*Cache, error) {

	client := getRedisClient(redisAddress)

	err := client.Ping(context.Background()).Err()
	if err != nil {
		return nil, fmt.Errorf("could not create cache: %w", err)
	}

	return &Cache{client: client, ttl: ttl}, nil
}

func (c *Cache) Add(ctx context.Context, key string, value interface{}) {
	c.client.Set(ctx, apqPrefix+key, value, c.ttl)
}

func (c *Cache) Get(ctx context.Context, key string) (interface{}, bool) {
	s, err := c.client.Get(ctx, apqPrefix+key).Result()
	if err != nil {
		return struct{}{}, false
	}
	return s, true
}

func accountingPubSubHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var msg PubSubMessage
		logger := config.GetLogger()

		// Redis lock is a best-effort optimization.
		// Reliability must not depend on Redis: we also serialize posting via MySQL advisory locks in ProcessMessage().
		redisLock := config.GetRedisLock()

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			config.LogError(logger, "server.go", "accountingPubSubHandler", "io.ReadAll", nil, err)
			// Malformed request body: ack/drop to avoid infinite retries.
			c.Status(http.StatusNoContent)
			return
		}

		// byte slice unmarshalling handles base64 decoding.
		if err := json.Unmarshal(body, &msg); err != nil {
			config.LogError(logger, "server.go", "accountingPubSubHandler", "Unmarshal body", body, err)
			// Malformed request: ack/drop to avoid infinite retries.
			c.Status(http.StatusNoContent)
			return
		}

		var m config.PubSubMessage
		if err := json.Unmarshal(msg.Message.Data, &m); err != nil {
			config.LogError(logger, "server.go", "accountingPubSubHandler", "Unmarshal pubsub message", msg.Message.Data, err)
			// Malformed Pub/Sub payload: ack/drop to avoid infinite retries.
			c.Status(http.StatusNoContent)
			return
		}

		// Basic validation to avoid retry loops on poisoned messages.
		if m.BusinessId == "" || m.ReferenceType == "" {
			config.LogError(logger, "server.go", "accountingPubSubHandler", "Invalid pubsub message (missing required fields)", m, fmt.Errorf("business_id/reference_type required"))
			c.Status(http.StatusNoContent)
			return
		}

		// Correlation ID propagation: prefer payload correlation_id; fall back to Pub/Sub message ID.
		correlationID := m.CorrelationId
		if correlationID == "" {
			correlationID = msg.Message.ID
		}

		// Best-effort: try to obtain a lock for the businessID to avoid long in-request blocking.
		// If Redis is unavailable / lock cannot be obtained, continue anyway; ProcessMessage() will serialize safely.
		var lock *redislock.Lock
		if redisLock == nil {
			logger.WithFields(logrus.Fields{
				"field":          "accountingPubSubHandler",
				"business_id":    m.BusinessId,
				"reference_type": m.ReferenceType,
				"reference_id":   m.ReferenceId,
				"message_id":     msg.Message.ID,
			}).Warn("redis lock not ready; proceeding without redis lock")
		} else {
			lock, err = redisLock.Obtain(c.Request.Context(), fmt.Sprintf("lock:%s", m.BusinessId), 30*time.Second, nil)
			if err == redislock.ErrNotObtained {
				logger.WithFields(logrus.Fields{
					"field":          "accountingPubSubHandler",
					"business_id":    m.BusinessId,
					"reference_type": m.ReferenceType,
					"reference_id":   m.ReferenceId,
					"message_id":     msg.Message.ID,
				}).Warn("could not obtain redis lock; proceeding without redis lock")
				lock = nil
			} else if err != nil {
				logger.WithFields(logrus.Fields{
					"field":          "accountingPubSubHandler",
					"business_id":    m.BusinessId,
					"reference_type": m.ReferenceType,
					"reference_id":   m.ReferenceId,
					"message_id":     msg.Message.ID,
				}).Warn("error obtaining redis lock; proceeding without redis lock: " + err.Error())
				lock = nil
			}
		}
		defer func() {
			if lock == nil {
				return
			}
			if releaseErr := lock.Release(c.Request.Context()); releaseErr != nil {
				logger.WithFields(logrus.Fields{
					"field":        "accountingPubSubHandler",
					"business_id":  m.BusinessId,
					"reference_id": m.ReferenceId,
					"message_id":   msg.Message.ID,
				}).Warn("failed to release redis lock: " + releaseErr.Error())
			}
		}()

		// Process the message
		ctx := context.WithValue(c.Request.Context(), utils.ContextKeyBusinessId, m.BusinessId)
		ctx = context.WithValue(ctx, utils.ContextKeyUserId, 0)
		ctx = context.WithValue(ctx, utils.ContextKeyUserName, "System")
		ctx = utils.SetCorrelationIdInContext(ctx, correlationID)
		if err := ProcessMessage(ctx, logger, m); err != nil {
			logger.WithFields(logrus.Fields{
				"field":          "accountingPubSubHandler",
				"business_id":    m.BusinessId,
				"reference_type": m.ReferenceType,
				"reference_id":   m.ReferenceId,
				"message_id":     msg.Message.ID,
				"correlation_id": correlationID,
			}).Error("pubsub processing failed: " + err.Error())
			// Non-2xx tells Pub/Sub to retry (and potentially route to DLQ).
			c.Status(http.StatusInternalServerError)
			return
		}

		// Success: ack.
		c.Status(http.StatusNoContent)
	}
}

// Defining the Graphql handler
func graphqlHandler() gin.HandlerFunc {
	// NewExecutableSchema and Config are in the generated.go file
	// Resolver is in the resolver.go file

	// IMPORTANT (Cloud Run): do not block startup waiting for Redis.
	// APQ cache is optional; if Redis isn't ready we run without it.
	logger := config.GetLogger()
	cache, err := NewCache(os.Getenv("REDIS_ADDRESS"), 24*time.Hour)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"field": "graphqlHandler",
		}).Warn("APQ redis cache disabled (redis not ready): " + err.Error())
		cache = nil
	}

	c := graph.Config{Resolvers: &graph.Resolver{
		Tracer: tracer,
	}}
	c.Directives.Auth = directives.Auth

	h := handler.NewDefaultServer(graph.NewExecutableSchema(c))
	h.Use(otelgqlgen.Middleware())
	h.AddTransport(transport.POST{})
	h.AddTransport(transport.MultipartForm{
		MaxMemory:     32 << 20, // 32 MB
		MaxUploadSize: 50 << 20, // 50 MB
	})
	if cache != nil {
		h.Use(extension.AutomaticPersistedQuery{Cache: cache})
	}
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// Defining the Playground handler
func playgroundHandler() gin.HandlerFunc {
	h := playground.Handler("GraphQL", "/query")

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// authorizeInternalBusiness ensures the session user is allowed to act on the provided business_id.
// - Admin users may act on any business.
// - Non-admin users may only act on their own business.
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

func authorizeAdminOnly(ctx context.Context) error {
	username, ok := utils.GetUsernameFromContext(ctx)
	if !ok || username == "" {
		return errors.New("unauthorized")
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
	if user.Role != models.UserRoleAdmin {
		return errors.New("unauthorized")
	}
	return nil
}

type outboxReplayRequest struct {
	BusinessId string `json:"business_id"`
	RecordId   int    `json:"record_id"`
}

func outboxReplayHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Require auth token (SessionMiddleware puts username in context).
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if err := authorizeAdminOnly(c.Request.Context()); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req outboxReplayRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.RecordId <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and record_id are required"})
			return
		}

		db := config.GetDB()
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "db is nil"})
			return
		}
		now := time.Now().UTC()
		if err := db.WithContext(c.Request.Context()).
			Model(&models.PubSubMessageRecord{}).
			Where("id = ? AND business_id = ?", req.RecordId, req.BusinessId).
			Updates(map[string]interface{}{
				"publish_status":     models.OutboxPublishStatusFailed,
				"next_attempt_at":    &now,
				"locked_at":          nil,
				"locked_by":          nil,
				"last_publish_error": nil,
			}).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"business_id":     req.BusinessId,
			"record_id":       req.RecordId,
			"publish_status":  models.OutboxPublishStatusFailed,
			"next_attempt_at": now.Format(time.RFC3339Nano),
		})
	}
}

type voidCloneSalesInvoiceRequest struct {
	BusinessId string `json:"business_id"`
	InvoiceId  int    `json:"invoice_id"`
}

func voidCloneSalesInvoiceHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Require auth token (SessionMiddleware puts username in context).
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var req voidCloneSalesInvoiceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.InvoiceId <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and invoice_id are required"})
			return
		}
		if err := authorizeInternalBusiness(c.Request.Context(), req.BusinessId); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), req.BusinessId)
		newInv, err := models.VoidAndCloneSalesInvoice(ctx, req.BusinessId, req.InvoiceId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cid, _ := utils.GetCorrelationIdFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{
			"new_invoice_id":     newInv.ID,
			"new_invoice_status": newInv.CurrentStatus,
			"old_invoice_id":     req.InvoiceId,
			"business_id":        req.BusinessId,
			"correlation_id":     cid,
		})
	}
}

type voidCloneBillRequest struct {
	BusinessId string `json:"business_id"`
	BillId     int    `json:"bill_id"`
}

func voidCloneBillHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req voidCloneBillRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.BillId <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and bill_id are required"})
			return
		}
		if err := authorizeInternalBusiness(c.Request.Context(), req.BusinessId); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), req.BusinessId)
		newBill, err := models.VoidAndCloneBill(ctx, req.BusinessId, req.BillId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cid, _ := utils.GetCorrelationIdFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{
			"new_bill_id":     newBill.ID,
			"new_bill_status": newBill.CurrentStatus,
			"old_bill_id":     req.BillId,
			"business_id":     req.BusinessId,
			"correlation_id":  cid,
		})
	}
}

type voidCloneSupplierCreditRequest struct {
	BusinessId       string `json:"business_id"`
	SupplierCreditId int    `json:"supplier_credit_id"`
}

func voidCloneSupplierCreditHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req voidCloneSupplierCreditRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.SupplierCreditId <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and supplier_credit_id are required"})
			return
		}
		if err := authorizeInternalBusiness(c.Request.Context(), req.BusinessId); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), req.BusinessId)
		newSC, err := models.VoidAndCloneSupplierCredit(ctx, req.BusinessId, req.SupplierCreditId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cid, _ := utils.GetCorrelationIdFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{
			"new_supplier_credit_id":     newSC.ID,
			"new_supplier_credit_status": newSC.CurrentStatus,
			"old_supplier_credit_id":     req.SupplierCreditId,
			"business_id":                req.BusinessId,
			"correlation_id":             cid,
		})
	}
}

type voidCloneCreditNoteRequest struct {
	BusinessId   string `json:"business_id"`
	CreditNoteId int    `json:"credit_note_id"`
}

func voidCloneCreditNoteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req voidCloneCreditNoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.CreditNoteId <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and credit_note_id are required"})
			return
		}
		if err := authorizeInternalBusiness(c.Request.Context(), req.BusinessId); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), req.BusinessId)
		newCN, err := models.VoidAndCloneCreditNote(ctx, req.BusinessId, req.CreditNoteId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cid, _ := utils.GetCorrelationIdFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{
			"new_credit_note_id":     newCN.ID,
			"new_credit_note_status": newCN.CurrentStatus,
			"old_credit_note_id":     req.CreditNoteId,
			"business_id":            req.BusinessId,
			"correlation_id":         cid,
		})
	}
}

type voidCloneSalesOrderRequest struct {
	BusinessId   string `json:"business_id"`
	SalesOrderId int    `json:"sales_order_id"`
}

func voidCloneSalesOrderHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req voidCloneSalesOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.SalesOrderId <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and sales_order_id are required"})
			return
		}
		if err := authorizeInternalBusiness(c.Request.Context(), req.BusinessId); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), req.BusinessId)
		newSO, err := models.CancelAndCloneSalesOrder(ctx, req.BusinessId, req.SalesOrderId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cid, _ := utils.GetCorrelationIdFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{
			"new_sales_order_id":     newSO.ID,
			"new_sales_order_status": newSO.CurrentStatus,
			"old_sales_order_id":     req.SalesOrderId,
			"business_id":            req.BusinessId,
			"correlation_id":         cid,
		})
	}
}

type voidClonePurchaseOrderRequest struct {
	BusinessId      string `json:"business_id"`
	PurchaseOrderId int    `json:"purchase_order_id"`
}

func voidClonePurchaseOrderHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req voidClonePurchaseOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.PurchaseOrderId <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and purchase_order_id are required"})
			return
		}
		if err := authorizeInternalBusiness(c.Request.Context(), req.BusinessId); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		ctx := utils.SetBusinessIdInContext(c.Request.Context(), req.BusinessId)
		newPO, err := models.CancelAndClonePurchaseOrder(ctx, req.BusinessId, req.PurchaseOrderId)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cid, _ := utils.GetCorrelationIdFromContext(ctx)
		c.JSON(http.StatusOK, gin.H{
			"new_purchase_order_id":     newPO.ID,
			"new_purchase_order_status": newPO.CurrentStatus,
			"old_purchase_order_id":     req.PurchaseOrderId,
			"business_id":               req.BusinessId,
			"correlation_id":            cid,
		})
	}
}

// Not supported yet (needs explicit reversal policy).
type voidCloneUnsupportedRequest struct {
	BusinessId string `json:"business_id"`
	Id         int    `json:"id"`
}

func voidCloneNotSupportedHandler(doc string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := utils.GetUsernameFromContext(c.Request.Context()); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		var req voidCloneUnsupportedRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.BusinessId == "" || req.Id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "business_id and id are required"})
			return
		}
		if err := authorizeInternalBusiness(c.Request.Context(), req.BusinessId); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("%s void+clone is not supported yet (needs explicit reversal policy)", doc),
		})
	}
}

func customNotFoundHandler(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
}

func main() {
	port := os.Getenv("API_PORT_2")
	if port == "" {
		// Cloud Run standard env var.
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = defaultPort
	}

	logger := config.GetLogger()

	// Shutdown coordination.
	// Cloud Run sends SIGTERM on revision shutdown; handle it for graceful drain.
	sigCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// Start the HTTP server ASAP so Cloud Run considers the revision healthy.
	// Until DB/Redis are ready, we return 503 for app endpoints.
	r := gin.New()
	// Correlation IDs: generate once per request and attach to context.
	r.Use(func(c *gin.Context) {
		cid := c.GetHeader("x-correlation-id")
		if cid == "" {
			cid = uuid.NewString()
		}
		c.Request = c.Request.WithContext(utils.SetCorrelationIdInContext(c.Request.Context(), cid))
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		// Always allow Cloud Run startup probe.
		if c.Request.URL.Path == "/healthz" {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		// Gate critical endpoints on dependency readiness.
		if config.GetDB() == nil || config.GetRedisDB() == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.Next()
	})

	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	// http.HandleFunc("/export", reports.ExportExcel)
	// err := http.ListenAndServe(":8084", nil)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// Setup UpTrace Logging with OpenTelemetry
	// upTraceDsn := os.Getenv("UPTRACE_DSN")
	// ctx := context.Background()
	// uptrace.ConfigureOpentelemetry(
	// 	uptrace.WithDSN(upTraceDsn),
	// )
	// defer uptrace.Shutdown(ctx)

	// var businesses []*models.Business
	// err := db.Select("id").Find(&businesses).Error
	// if err != nil {
	// 	logger.WithFields(logrus.Fields{
	// 		"field": "Connecting to Database",
	// 	}).Panic(err.Error())
	// }
	// log.Println("Connected to Database")
	// for _, business := range businesses {
	// 	err = RunAccountingWorkflow(business.ID.String())
	// 	if err != nil {
	// 		logger.WithFields(logrus.Fields{
	// 			"field": "Starting Accounting Workflow",
	// 		}).Panic(err.Error())
	// 	}
	// }

	// client := getRedisClient(os.Getenv("REDIS_ADDRESS"))
	// Initialize RateLimiter instance.
	// rateLimiter := NewRateLimiter(client, 10, time.Minute) //  10 requests per minute

	// Initialize remaining router (after readiness gate is installed).

	// Apply rate limiting middleware to all routes.
	// r.Use(rateLimiter.RateLimitMiddleware)

	corsConfig := cors.DefaultConfig()
	// Production-safe CORS:
	// - In production, require explicit allowlist via CORS_ALLOWED_ORIGINS (comma-separated).
	// - In non-production, allow all (developer convenience).
	allowedOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GO_ENV")), "production") {
		if allowedOrigins == "" {
			// Safer default: deny all if not configured in production.
			corsConfig.AllowOrigins = []string{}
		} else {
			corsConfig.AllowOrigins = splitAndTrim(allowedOrigins)
		}
	} else {
		corsConfig.AllowAllOrigins = true
	}
	corsConfig.AddAllowMethods("GET", "POST", "PUT", "DELETE", "OPTIONS")
	corsConfig.AddAllowHeaders("token", "Origin", "Content-Type", "Authorization")
	corsConfig.AddExposeHeaders("Content-Length")
	corsConfig.AllowCredentials = true

	r.Use(cors.New(corsConfig))
	// r.Use(middlewares.AuthMiddleware())

	// Optional rate limiting (recommended for production).
	// Env:
	// - RATE_LIMIT_ENABLED=true
	// - RATE_LIMIT_WINDOW_SECONDS=60
	// - RATE_LIMIT_MAX_REQUESTS=600
	if strings.EqualFold(strings.TrimSpace(os.Getenv("RATE_LIMIT_ENABLED")), "true") {
		client := getRedisClient(os.Getenv("REDIS_ADDRESS"))
		limit := int64(600)
		if v := strings.TrimSpace(os.Getenv("RATE_LIMIT_MAX_REQUESTS")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				limit = n
			}
		}
		windowSec := int64(60)
		if v := strings.TrimSpace(os.Getenv("RATE_LIMIT_WINDOW_SECONDS")); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				windowSec = n
			}
		}
		rateLimiter := NewRateLimiter(client, limit, time.Duration(windowSec)*time.Second)
		r.Use(rateLimiter.RateLimitMiddleware)
	}

	r.Use(middlewares.SessionMiddleware())
	r.Use(middlewares.LoaderMiddleware())
	r.Use(customErrorLogger(logger))
	r.Use(gin.Recovery())
	r.POST("/query", graphqlHandler())
	r.GET("/", playgroundHandler())
	r.POST("/pubsub", accountingPubSubHandler())
	// Ops tooling (admin only): replay outbox messages that were marked DEAD/FAILED.
	r.POST("/internal/ops/outbox/replay", outboxReplayHandler())
	// Internal helper flow: void + clone (draft) for immutable inventory docs.
	r.POST("/internal/void-clone/sales-invoice", voidCloneSalesInvoiceHandler())
	r.POST("/internal/void-clone/bill", voidCloneBillHandler())
	r.POST("/internal/void-clone/supplier-credit", voidCloneSupplierCreditHandler())
	r.POST("/internal/void-clone/credit-note", voidCloneCreditNoteHandler())
	// Orders are cancel+clone (no posting/outbox).
	r.POST("/internal/void-clone/sales-order", voidCloneSalesOrderHandler())
	r.POST("/internal/void-clone/purchase-order", voidClonePurchaseOrderHandler())
	// Not supported yet: requires explicit reversal policy.
	r.POST("/internal/void-clone/transfer-order", voidCloneNotSupportedHandler("transfer order"))
	r.POST("/internal/void-clone/inventory-adjustment", voidCloneNotSupportedHandler("inventory adjustment"))
	// go RunAccountingWorkflow()
	r.NoRoute(customNotFoundHandler)

	// Start listening immediately (Cloud Run startup probe is TCP based).
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	serverErrCh := make(chan error, 1)
	go func() {
		// ListenAndServe returns http.ErrServerClosed on graceful shutdown.
		serverErrCh <- srv.ListenAndServe()
	}()

	// Connect dependencies after the port is open.
	config.ConnectDatabaseWithRetry()
	config.ConnectRedisWithRetry()

	// Now DB is ready; run migrations.
	db := config.GetDB()
	sqlDB, _ := db.DB()
	defer func() {
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}()
	// IMPORTANT: AutoMigrate can run DDL that blocks tables and causes 504/502 timeouts.
	// Allow disabling migrations on startup (run them as a separate job instead).
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("SKIP_MIGRATIONS")), "true") {
		models.MigrateTable()
	} else {
		logger.WithFields(logrus.Fields{"field": "migrations"}).Warn("SKIP_MIGRATIONS=true; skipping AutoMigrate on startup")
	}

	// Start outbox dispatcher (publishes AFTER commit).
	dispatcherCtx, cancelDispatcher := context.WithCancel(context.Background())
	defer cancelDispatcher()
	go workflow.NewOutboxDispatcher(db, logger).Run(dispatcherCtx)

	// Set the session isolation level to READ COMMITTED
	for attempt := 1; ; attempt++ {
		err := db.Exec("SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED").Error
		if err == nil {
			break
		}
		sleep := time.Second * time.Duration(1<<min(attempt, 5))
		if sleep > 30*time.Second {
			sleep = 30 * time.Second
		}
		logger.WithFields(logrus.Fields{
			"field":   "database",
			"attempt": attempt,
		}).Warn("failed to set isolation level; retrying in " + sleep.String() + ": " + err.Error())
		time.Sleep(sleep)
	}

	logger.WithFields(logrus.Fields{
		"info": "Connection Established",
	}).Info("connect to http://localhost:", port, "/ for GraphQL playground")
	log.Println("Server started successfully")

	// Block until shutdown or server error.
	select {
	case <-sigCtx.Done():
		// graceful shutdown below
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.WithFields(logrus.Fields{"field": "http"}).Error("server stopped unexpectedly: " + err.Error())
		}
	}

	// Stop background workers first so they don't start new work while we're draining.
	cancelDispatcher()

	// Drain HTTP requests.
	shutdownTimeout := 30 * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithFields(logrus.Fields{"field": "http"}).Error("graceful shutdown failed: " + err.Error())
	}

	// Close Redis (best-effort).
	if rdb := config.GetRedisDB(); rdb != nil {
		_ = rdb.Close()
	}
}

// customErrorLogger is a custom Gin middleware that logs only errors
func customErrorLogger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Only log when there are errors
		if len(c.Errors) > 0 {
			logger.Error(c.Errors.String())
		}
	}
}

// Initialize a new RateLimiter instance.
func NewRateLimiter(client *redis.Client, limit int64, window time.Duration) *RateLimiter {
	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

// Middleware function to check rate limits.
func (rl *RateLimiter) RateLimitMiddleware(c *gin.Context) {
	// Get the IP address or user identifier from the request.
	key := c.ClientIP() // Assuming IP-based rate limiting

	// Check if the key exists in Redis.
	exists, err := rl.client.Exists(c.Request.Context(), key).Result()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// If the key doesn't exist, create it and set expiry.
	if exists == 0 {
		err := rl.client.Set(c.Request.Context(), key, 1, rl.window).Err()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Next()
		return
	}

	// If the key exists, get the current count.
	count, err := rl.client.Incr(c.Request.Context(), key).Result()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// If the count exceeds the limit, return an error response.
	if count > rl.limit {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": fmt.Sprintf("Rate limit exceeded. Try again in %d seconds", int(rl.window.Seconds())),
		})
		return
	}

	c.Next()
}

func splitAndTrim(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
