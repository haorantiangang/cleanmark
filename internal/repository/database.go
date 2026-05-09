package repository

import (
	"cleanmark/internal/model"
	"cleanmark/config"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var testDB *gorm.DB

func Init(cfg *config.DatabaseConfig) error {
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := autoMigrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

func autoMigrate() error {
	return DB.AutoMigrate(
		model.User{},
		model.Task{},
		model.Order{},
	)
}

func GetDB() *gorm.DB {
	return DB
}

func SetTestDB(db *gorm.DB) {
	DB = db
}

func InitTestDB() *gorm.DB {
	var err error
	testDB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("Failed to create test database: " + err.Error())
	}

	err = testDB.AutoMigrate(&model.User{}, &model.Task{}, &model.Order{})
	if err != nil {
		panic("Failed to migrate test database: " + err.Error())
	}

	DB = testDB
	return testDB
}

func GetTestDB() *gorm.DB {
	return testDB
}
