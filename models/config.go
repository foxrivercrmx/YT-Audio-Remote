package models

import "gorm.io/gorm"

// AppConfig menyimpan pengaturan aplikasi (key-value)
type AppConfig struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex"`
	Value string
}