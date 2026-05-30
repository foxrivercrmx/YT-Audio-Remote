package models

import "gorm.io/gorm"

// Queue menyimpan data antrian lagu dari YouTube
type Queue struct {
	gorm.Model
	VideoID   string // ID asli YouTube (misal: dQw4w9WgXcQ)
	Title     string // Judul lagu/video
	Artist    string // Nama channel atau uploader
	Duration  int    // Durasi dalam detik
	Thumbnail string // URL gambar thumbnail
	Status    string `gorm:"default:'waiting'"` // Status: 'waiting', 'playing', 'done'
	DirectURL string // URL audio stream asli (diisi otomatis oleh server sesaat sebelum diputar)
}