package database

import (
	"log"
	"yt-audio-remote/models" // Sesuaikan nama module

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB(dbPath string) {
	var err error
	
	// Konfigurasi database murni tanpa CGO
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Matikan log query agar terminal bersih
	})

	if err != nil {
		log.Fatal("Gagal koneksi ke database SQLite:", err)
	}

	log.Println("Database terhubung. Menjalankan AutoMigrate...")
	
	// Migrasi tabel otomatis
	err = DB.AutoMigrate(&models.AppConfig{}, &models.Queue{})
	if err != nil {
		log.Fatal("Gagal migrasi database:", err)
	}
	
	seedDefaultConfig()
}

// seedDefaultConfig menyisipkan nilai awal ke tabel AppConfig jika belum ada
func seedDefaultConfig() {
	defaultConfigs := []models.AppConfig{
		{Key: "audio_codec", Value: "m4a"},    // m4a, webm, opus
		{Key: "audio_bitrate", Value: "best"}, // best, 128, 192, dll
	}

	for _, config := range defaultConfigs {
		DB.Where(models.AppConfig{Key: config.Key}).FirstOrCreate(&config)
	}
	log.Println("Seeding konfigurasi default selesai.")
}