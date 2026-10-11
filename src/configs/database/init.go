package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	xiaozhi_utils "xiaozhi-server-go/src/core/utils"

	gorm_logger "gorm.io/gorm/logger"
)

const (
	SystemConfigID = 1 // System config ID
	ModuleConfigID = 1 // Module config ID
	ServerConfigID = 1 // Server config ID
	ServerStatusID = 1 // Server status ID
)

type DBLogger struct {
	logger *xiaozhi_utils.Logger
}

func (l *DBLogger) LogMode(level gorm_logger.LogLevel) gorm_logger.Interface {
	return &DBLogger{
		logger: l.logger,
	}
}

func (l *DBLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	l.logger.Info(msg, data...)
}

func (l *DBLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	l.logger.Warn(msg, data...)
}

func (l *DBLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	l.logger.Error(msg, data...)
}

func (l *DBLogger) Trace(
	ctx context.Context,
	begin time.Time,
	fc func() (sql string, rowsAffected int64),
	err error,
) {
	sql, rows := fc()
	elapsed := time.Since(begin)
	if err != nil {
		if strings.Contains(err.Error(), "record not found") {
			// Ignore record not found errors
			return
		}
		l.logger.Error("SQL Trace Error", map[string]interface{}{
			"sql":     sql,
			"rows":    rows,
			"elapsed": elapsed,
			"err":     err,
		})
	} else {
		l.logger.Debug("SQL Trace", map[string]interface{}{
			"sql":     sql,
			"rows":    rows,
			"elapsed": elapsed,
		})
	}
}

var (
	DB       *gorm.DB
	dbLogger *xiaozhi_utils.Logger
	dbType   string
)

func GetDB() *gorm.DB {
	if DB == nil {
		panic("database not initialized, please call InitDB() first")
	}
	return DB
}

// GetTxDB returns a new transaction DB instance (requires manual Commit or Rollback)
func GetTxDB() *gorm.DB {
	if DB == nil {
		panic("database not initialized, please call InitDB() first")
	}
	return DB.Begin()
}

// InitDB initializes database connection and migrates tables
func InitDB() (*gorm.DB, string, error) {
	var (
		db  *gorm.DB
		err error
	)

	dbType = "sqlite"
	path := "./config.db"
	db, err = gorm.Open(sqlite.Open(path))

	if err != nil {
		return nil, "", fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate tables
	if err := migrateTables(db); err != nil {
		return nil, dbType, err
	}

	DB = db
	NewServerConfigDB(db)
	return db, dbType, nil
}

func SetLogger(logger *xiaozhi_utils.Logger) {
	dbLogger = logger
	DB.Logger = &DBLogger{logger: logger}

	var version string
	DB.Raw("SELECT sqlite_version()").Scan(&version)
	logger.Info("SQLite database connected successfully, version: %s", version)
}

// migrateTables runs automatic migration for database models
func migrateTables(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.LLMConfig{},
		&models.TTSConfig{},
		&models.ASRConfig{},
		&models.VLLLMConfig{},
		&models.User{},
		&models.Agent{},
		&models.AgentDialog{},
		&models.Device{},
		&models.AuthClient{},
		&models.ServerStatus{},
	)
	return err
}

// InsertDefaultConfigIfNeeded inserts default configurations on first boot
func InsertDefaultConfigIfNeeded(db *gorm.DB) error {
	config := configs.Cfg
	if err := InitProviders(db, config); err != nil {
		return fmt.Errorf("failed to initialize providers configuration: %v", err)
	}
	if err := InitAdminUser(db, config); err != nil {
		return fmt.Errorf("failed to initialize admin user: %v", err)
	}
	if err := NormalizeSingleUserData(db); err != nil {
		return fmt.Errorf("failed to normalize single-user data: %v", err)
	}
	return nil
}
