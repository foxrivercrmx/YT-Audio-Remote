package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"os/exec"
	"time"

	"github.com/gofiber/fiber/v2"

	// Sesuaikan nama module dengan go.mod kamu
	"yt-audio-remote/database"
	"yt-audio-remote/models"
)

// YTDLPResult merepresentasikan struktur metadata JSON dari yt-dlp
type YTDLPResult struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Uploader  string  `json:"uploader"`
	Channel   string  `json:"channel"`
	Duration  int     `json:"duration"` // --flat-playlist biasanya mengembalikan detik bulat
	Thumbnail string  `json:"thumbnail"`
	Thumbnails []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}

var (
	YTJSRuntime   string
	YTCookiesFile string
)

// runYTDLP adalah fungsi inti pembungkus yt-dlp
func runYTDLP(args ...string) ([]YTDLPResult, error) {
	// Base argumen: ambil metadata saja (json), jangan download videonya, 
	// dan menyamar sebagai klien mobile untuk menghindari po_token.
	baseArgs := []string{
		"--dump-json",
		"--flat-playlist",
		"--extractor-args", "youtube:player_client=android,ios,tv",
	}

	// 1. Injeksi JS Runtimes jika ada
	if YTJSRuntime != "" {
		baseArgs = append(baseArgs, "--js-runtimes", YTJSRuntime)
	}

	// 2. Injeksi Cookies jika filenya tersedia
	if YTCookiesFile != "" {
		baseArgs = append(baseArgs, "--cookies", YTCookiesFile)
	}

	finalArgs := append(baseArgs, args...)
	cmd := exec.Command("yt-dlp", finalArgs...)

	out, err := cmd.Output()
	if err != nil {
		log.Println("Gagal eksekusi yt-dlp:", err)
		return nil, err
	}

	var results []YTDLPResult

	// yt-dlp --dump-json akan mengeluarkan satu baris JSON utuh per video/lagu.
	// Jika yang diinput adalah playlist, akan ada banyak baris (JSON Lines).
	scanner := bufio.NewScanner(bytes.NewReader(out))
	// Diperbesar kapasitas buffernya untuk berjaga-jaga jika metadata JSON sangat panjang
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var res YTDLPResult
		if err := json.Unmarshal(line, &res); err != nil {
			log.Println("Gagal parsing JSON baris yt-dlp:", err)
			continue
		}

		// ==========================================
		// 1. FILTER: Buang Video yang Dihapus / Private
		// ==========================================
		if res.Title == "[Deleted video]" || res.Title == "[Private video]" || res.Title == "" {
			continue // Lewati dan jangan masukkan ke dalam daftar hasil
		}

		// ==========================================
		// 2. FIX COVER: Tangani Format Playlist
		// ==========================================
		// Jika thumbnail string kosong, tapi ada data di array thumbnails
		if res.Thumbnail == "" && len(res.Thumbnails) > 0 {
			// Ambil resolusi gambar yang paling terakhir (biasanya yt-dlp mengurutkan yang terbesar di akhir)
			res.Thumbnail = res.Thumbnails[len(res.Thumbnails)-1].URL
		}

		// Antisipasi perbedaan field antara YouTube Music dan YouTube biasa
		if res.Uploader == "" && res.Channel != "" {
			res.Uploader = res.Channel
		}

		results = append(results, res)
	}

	return results, nil
}

// SearchYouTube menangani query pencarian teks dari remote
func SearchYouTube(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Parameter pencarian (q) kosong"})
	}

	// Membatasi hasil pencarian 15 lagu saja agar enteng
	searchArg := "ytsearch15:" + query
	results, err := runYTDLP(searchArg)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal mencari di YouTube"})
	}

	return c.JSON(fiber.Map{"status": "ok", "data": results})
}

// ParseURL menangani URL spesifik (music.youtube.com, Playlist, Video biasa)
func ParseURL(c *fiber.Ctx) error {
	var body struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&body); err != nil || body.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL tidak valid"})
	}

	// Cukup oper URL-nya, runYTDLP (lewat --flat-playlist) akan otomatis menjabarkannya
	// Entah itu 1 lagu direct, atau 100 lagu dalam satu playlist
	results, err := runYTDLP(body.URL)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal memproses URL"})
	}

	return c.JSON(fiber.Map{"status": "ok", "data": results})
}

// AddQueue menyimpan lagu (atau kumpulan lagu) ke database dengan status waiting
func AddQueue(c *fiber.Ctx) error {
	var payload struct {
		Items []models.Queue `json:"items"`
	}
	
	if err := c.BodyParser(&payload); err != nil || len(payload.Items) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Data antrian tidak valid"})
	}

	// GORM mendukung bulk insert secara otomatis jika kita mengoper slice
	if err := database.DB.Create(&payload.Items).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal menyimpan ke database"})
	}

	return c.JSON(fiber.Map{"status": "ok", "message": "Berhasil ditambahkan ke antrian"})
}

// GetQueue mengambil daftar antrian yang belum selesai diputar
func GetQueue(c *fiber.Ctx) error {
	var queues []models.Queue
	
	// Ambil semua yang bukan "done", urutkan berdasarkan waktu dimasukkan
	database.DB.Where("status != ?", "done").Order("created_at asc").Find(&queues)
	
	return c.JSON(fiber.Map{"status": "ok", "data": queues})
}

// DeleteQueue menghapus lagu spesifik dari antrian (bisa dipakai untuk tombol 'Hapus' di remote)
func DeleteQueue(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(400).JSON(fiber.Map{"error": "ID tidak valid"})
	}

	database.DB.Delete(&models.Queue{}, id)
	return c.JSON(fiber.Map{"status": "ok"})
}

// ReorderQueue mengatur ulang urutan lagu di database
func ReorderQueue(c *fiber.Ctx) error {
	var payload struct {
		IDs []uint `json:"ids"` // Menerima array ID dengan urutan yang baru dari HP
	}

	if err := c.BodyParser(&payload); err != nil || len(payload.IDs) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Data urutan tidak valid"})
	}

	// Waktu dasar diambil saat ini
	baseTime := time.Now()

	// Gunakan transaksi agar proses update massal jauh lebih cepat dan aman
	tx := database.DB.Begin()
	
	for i, id := range payload.IDs {
		// Setiap lagu ditambahkan sekian detik sesuai urutan posisinya di array.
		// Hasilnya: lagu urutan pertama waktunya lebih lawas dibanding urutan kedua.
		newTime := baseTime.Add(time.Duration(i) * time.Second)
		tx.Model(&models.Queue{}).Where("id = ?", id).Update("created_at", newTime)
	}
	
	tx.Commit()

	return c.JSON(fiber.Map{"status": "ok", "message": "Urutan berhasil diperbarui"})
}

// ClearQueue menghapus semua lagu yang antre, termasuk yang mengalami error
func ClearQueue(c *fiber.Ctx) error {
	// GORM: Menghapus massal lagu dengan status 'waiting' ATAU 'error'
	if err := database.DB.Where("status IN ?", []string{"waiting", "error"}).Delete(&models.Queue{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Gagal membersihkan antrian"})
	}

	return c.JSON(fiber.Map{"status": "ok", "message": "Antrian berhasil dibersihkan"})
}

// GetConfig mengambil pengaturan saat ini (codec & bitrate)
func GetConfig(c *fiber.Ctx) error {
	var configs []models.AppConfig
	database.DB.Find(&configs)
	
	// Ubah array struct ke bentuk map (key-value) agar mudah dibaca Javascript
	configMap := make(map[string]string)
	for _, conf := range configs {
		configMap[conf.Key] = conf.Value
	}
	return c.JSON(fiber.Map{"status": "ok", "data": configMap})
}

// UpdateConfig memperbarui pengaturan dari UI Remote
func UpdateConfig(c *fiber.Ctx) error {
	var payload map[string]string
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Data tidak valid"})
	}

	tx := database.DB.Begin()
	for key, value := range payload {
		tx.Model(&models.AppConfig{}).Where("key = ?", key).Update("value", value)
	}
	tx.Commit()

	return c.JSON(fiber.Map{"status": "ok", "message": "Pengaturan disimpan"})
}