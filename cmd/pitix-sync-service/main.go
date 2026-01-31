package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/middlewares"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/pitixsync"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/sirupsen/logrus"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PITIX_SYNC_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = defaultPort
	}

	logger := config.GetLogger()

	sigCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	r := gin.New()
	r.Use(func(c *gin.Context) {
		cid := c.GetHeader("x-correlation-id")
		if cid == "" {
			cid = uuid.NewString()
		}
		c.Request = c.Request.WithContext(utils.SetCorrelationIdInContext(c.Request.Context(), cid))
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		if config.GetDB() == nil || config.GetRedisDB() == nil {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.Next()
	})
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	corsConfig := cors.DefaultConfig()
	allowedOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GO_ENV")), "production") {
		if allowedOrigins == "" {
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
	r.Use(func(c *gin.Context) {
		if c.GetHeader("token") == "" {
			auth := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				token := strings.TrimSpace(auth[7:])
				if token != "" {
					c.Request.Header.Set("token", token)
				}
			}
		}
		c.Next()
	})
	r.Use(middlewares.SessionMiddleware())
	r.Use(customErrorLogger(logger))
	r.Use(gin.Recovery())

	// API endpoints (PitiX Sync)
	r.GET("/api/integrations/pitix/status", pitixsync.StatusHandler())
	r.POST("/api/integrations/pitix/connect", pitixsync.ConnectHandler())
	r.POST("/api/integrations/pitix/disconnect", pitixsync.DisconnectHandler())
	r.POST("/api/integrations/pitix/settings", pitixsync.UpdateSettingsHandler())
	r.POST("/api/integrations/pitix/sync", pitixsync.TriggerSyncHandler())
	r.GET("/api/integrations/pitix/sync-runs", pitixsync.SyncHistoryHandler())
	r.GET("/api/integrations/pitix/sync-runs/:id", pitixsync.SyncRunDetailHandler())
	r.POST("/api/integrations/pitix/sync-runs/:id/retry", pitixsync.RetrySyncRunHandler())

	// Pub/Sub push endpoint for sync worker.
	r.POST("/pubsub/pitix-sync", pitixsync.PubSubPushHandler())

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- srv.ListenAndServe()
	}()

	config.ConnectDatabaseWithRetry()
	config.ConnectRedisWithRetry()

	db := config.GetDB()
	sqlDB, _ := db.DB()
	defer func() {
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}()

	if !strings.EqualFold(strings.TrimSpace(os.Getenv("SKIP_MIGRATIONS")), "true") {
		models.MigrateTable()
	} else {
		logger.WithFields(logrus.Fields{"field": "migrations"}).Warn("SKIP_MIGRATIONS=true; skipping AutoMigrate on startup")
	}

	select {
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	case err := <-serverErrCh:
		if err != nil && err != http.ErrServerClosed {
			logger.WithFields(logrus.Fields{"field": "server"}).Error(err)
		}
	}
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

func customErrorLogger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		cid, _ := utils.GetCorrelationIdFromContext(c.Request.Context())
		logger.WithFields(logrus.Fields{
			"status":         c.Writer.Status(),
			"method":         c.Request.Method,
			"path":           c.Request.URL.Path,
			"latency":        latency.String(),
			"correlation_id": cid,
		}).Info("request")
	}
}

func intFromEnv(key string, def int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return n
}
