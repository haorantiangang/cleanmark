package tests

import (
	"cleanmark/internal/model"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("无法创建测试数据库: %v", err)
	}

	err = db.AutoMigrate(&model.User{}, &model.Task{}, &model.Order{})
	if err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	return db
}

type TestDB struct {
	*gorm.DB
}

func InitTestDB() *TestDB {
	var err error
	testDB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("无法创建测试数据库: " + err.Error())
	}

	err = testDB.AutoMigrate(&model.User{}, &model.Task{}, &model.Order{})
	if err != nil {
		panic("数据库迁移失败: " + err.Error())
	}

	return &TestDB{testDB}
}

func GetTestDB() *TestDB {
	return &TestDB{testDB}
}
