package database

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init initializes the database connection and runs migrations
func Init(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// Run migrations
	if err := db.AutoMigrate(
		&User{},
		&ProviderConfig{},
		&APIKey{},
		&UsageRecord{},
	); err != nil {
		return nil, err
	}

	log.Println("Database initialized successfully")
	return db, nil
}
