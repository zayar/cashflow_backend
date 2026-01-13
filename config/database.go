package config

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const SearchLimit = 10

var (
	db *gorm.DB
)

func GetDB() *gorm.DB {
	return db
}

func init() {
	// Load env from .env
	godotenv.Load()
	// IMPORTANT (Cloud Run):
	// Do NOT block startup in init() waiting for DB.
	// Cloud Run requires the container to start listening on $PORT quickly.
}

// ConnectDatabaseWithRetry connects and sets the global DB.
// Call this from main() AFTER the HTTP server is listening.
func ConnectDatabaseWithRetry() {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME_2")

	network := "tcp"
	address := fmt.Sprintf("%s:%s", dbHost, dbPort)

	// Cloud Run + Cloud SQL: when DB_HOST is "/cloudsql/<CONNECTION_NAME>",
	// connect using a Unix domain socket provided by Cloud SQL Auth Proxy.
	//
	// Example:
	//   DB_HOST=/cloudsql/cashflow-483906:asia-southeast1:cashflow-mysql-dev-upgrade
	if strings.HasPrefix(dbHost, "/cloudsql/") {
		network = "unix"
		address = dbHost
	}

	databaseConfig := fmt.Sprintf("%s:%s@%s(%s)/%s?multiStatements=true&parseTime=true",
		dbUser,
		dbPassword,
		network,
		address,
		dbName,
	)

	var attempt int
	for {
		attempt++
		var err error
		db, err = gorm.Open(mysql.Open(databaseConfig), initConfig())
		if err == nil {
			// Tune database/sql pool for Cloud SQL / production.
			// Env overrides (optional):
			// - DB_MAX_OPEN_CONNS (default 50)
			// - DB_MAX_IDLE_CONNS (default 25)
			// - DB_CONN_MAX_LIFETIME_SECONDS (default 300)
			// - DB_CONN_MAX_IDLE_TIME_SECONDS (default 60)
			if sqlDB, derr := db.DB(); derr == nil && sqlDB != nil {
				maxOpen := intFromEnv("DB_MAX_OPEN_CONNS", 50)
				maxIdle := intFromEnv("DB_MAX_IDLE_CONNS", 25)
				connMaxLife := time.Duration(intFromEnv("DB_CONN_MAX_LIFETIME_SECONDS", 300)) * time.Second
				connMaxIdle := time.Duration(intFromEnv("DB_CONN_MAX_IDLE_TIME_SECONDS", 60)) * time.Second

				if maxOpen > 0 {
					sqlDB.SetMaxOpenConns(maxOpen)
				}
				if maxIdle >= 0 {
					sqlDB.SetMaxIdleConns(maxIdle)
				}
				if connMaxLife > 0 {
					sqlDB.SetConnMaxLifetime(connMaxLife)
				}
				if connMaxIdle > 0 {
					sqlDB.SetConnMaxIdleTime(connMaxIdle)
				}
			}

			if pluginErr := db.Use(otelgorm.NewPlugin()); pluginErr != nil {
				log.Printf("db connected but failed to install otelgorm plugin: %v", pluginErr)
			}
			if pluginErr := db.Use(NewTenantGuardPlugin()); pluginErr != nil {
				log.Printf("db connected but failed to install tenant guard plugin: %v", pluginErr)
			}
			log.Printf("connected to database (attempt=%d)", attempt)
			return
		}

		sleep := time.Second * time.Duration(1<<min(attempt, 5))
		if sleep > 30*time.Second {
			sleep = 30 * time.Second
		}
		log.Printf("failed to connect database (attempt=%d): %v; retrying in %s", attempt, err, sleep)
		time.Sleep(sleep)
	}
}

func intFromEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// InitConfig Initialize Config
func initConfig() *gorm.Config {
	return &gorm.Config{
		// Logger: WriteGormLog(),
		Logger:         initLog(),
		NamingStrategy: initNamingStrategy(),
	}
}

// InitLog Connection Log Configuration
func initLog() logger.Interface {
	// f, _ := os.Create("gorm.log")
	// newLogger := logger.New(log.New(io.MultiWriter(f), "\r\n", log.LstdFlags), logger.Config{
	// 	Colorful:      true,
	// 	LogLevel:      logger.Error,
	// 	SlowThreshold: time.Second,
	// })
	// return newLogger
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // Output to standard output
		logger.Config{
			Colorful:      false,
			LogLevel:      logger.Error, // Adjust log level as needed
			SlowThreshold: time.Second,
		},
	)
	return newLogger
}

// InitNamingStrategy Init NamingStrategy
func initNamingStrategy() *schema.NamingStrategy {
	return &schema.NamingStrategy{
		SingularTable: false,
		TablePrefix:   "",
	}
}

func WriteGormLog() logger.Interface {
	logFile := os.Getenv("GORM_LOG")
	if logFile == "" {
		return initLog()
	}
	f, _ := os.Create(logFile)
	// f, _ := os.Create("gorm.log")
	newLogger := logger.New(log.New(io.MultiWriter(f), "\r\n", log.LstdFlags), logger.Config{
		Colorful:      true,
		LogLevel:      logger.Info,
		SlowThreshold: time.Second,
	})
	return newLogger
}
