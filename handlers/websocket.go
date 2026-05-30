package handlers

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/gofiber/websocket/v2"

	// Sesuaikan nama module dengan go.mod kamu
	"yt-audio-remote/database"
	"yt-audio-remote/models"
)

// WSMessage mendefinisikan format baku komunikasi JSON antara Server, Remote, dan Player
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

var (
	// Menyimpan klien yang aktif
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex // Mutex mencegah bentrok memori (race condition) saat nambah/hapus klien
)

// broadcast mengirim pesan ke seluruh klien yang terhubung (Player maupun Remote)
func broadcast(msg WSMessage) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		err := client.WriteJSON(msg)
		if err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}

// getDirectURL memanggil yt-dlp -g khusus untuk mendapatkan link stream audio m4a/terbaik
func getDirectURL(videoID string) (string, error) {
	url := "https://www.youtube.com/watch?v=" + videoID
	
	// 1. Ambil nilai Codec dan Bitrate dari Database
	var codecConfig, bitrateConfig models.AppConfig
	database.DB.Where("key = ?", "audio_codec").First(&codecConfig)
	database.DB.Where("key = ?", "audio_bitrate").First(&bitrateConfig)

	// Beri nilai default jika terjadi kegagalan baca DB
	codec := "m4a"
	if codecConfig.Value != "" {
		codec = codecConfig.Value
	}
	bitrate := "best"
	if bitrateConfig.Value != "" {
		bitrate = bitrateConfig.Value
	}

	// 2. Rakit format string yt-dlp secara dinamis
	var formatArg string
	if bitrate == "best" {
		formatArg = fmt.Sprintf("bestaudio[ext=%s]/bestaudio/best", codec)
	} else {
		formatArg = fmt.Sprintf("bestaudio[ext=%s][abr<=%s]/bestaudio[ext=%s]/bestaudio/best", codec, bitrate, codec)
	}

	// 3. Susun argumen dasar
	args := []string{
		"-g",
		"-f", formatArg,
		"--extractor-args", "youtube:player_client=android,ios,tv",
	}

	// 1. Injeksi JS Runtimes jika ada
	if YTJSRuntime != "" {
		args = append(args, "--js-runtimes", YTJSRuntime)
	}

	// 2. Injeksi Cookies jika filenya tersedia
	if YTCookiesFile != "" {
		args = append(args, "--cookies", YTCookiesFile)
	}

	args = append(args, url)

	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(out)), nil
}

// Fungsi pembantu baru: Melakukan ekstraksi JIT dan broadcast perintah PLAY
func extractAndPlay(song models.Queue) {
	log.Printf("Mengekstrak audio stream untuk: %s...", song.Title)

	directURL, err := getDirectURL(song.VideoID)
	if err != nil || directURL == "" {
		log.Println("Gagal mendapatkan link stream. Melewati lagu:", err)
		song.Status = "error"
		database.DB.Save(&song)
		ProcessNextSong() // Langsung lompat ke lagu berikutnya
		return
	}

	song.DirectURL = directURL
	database.DB.Save(&song)
	broadcast(WSMessage{Type: "PLAY", Payload: song})
}

// ProcessNextSong adalah mesin utama yang mengatur pergeseran antrian
func ProcessNextSong() {
	database.DB.Model(&models.Queue{}).Where("status = ?", "playing").Update("status", "done")

	var nextSong models.Queue
	result := database.DB.Where("status = ?", "waiting").Order("created_at asc").First(&nextSong)

	if result.Error != nil {
		broadcast(WSMessage{Type: "QUEUE_EMPTY"})
		return
	}

	nextSong.Status = "playing"
	database.DB.Save(&nextSong)
	
	extractAndPlay(nextSong)
}

// WebSocketHub adalah gerbang utama yang menangani semua koneksi WS masuk
func WebSocketHub(c *websocket.Conn) {
	// Daftarkan klien baru
	clientsMu.Lock()
	clients[c] = true
	clientsMu.Unlock()
	log.Println("Client WS terhubung. Total:", len(clients))

	defer func() {
		clientsMu.Lock()
		delete(clients, c)
		clientsMu.Unlock()
		c.Close()
		log.Println("Client WS terputus. Total:", len(clients))
	}()

	// Loop utama membaca pesan dari klien
	for {
		var msg WSMessage
		if err := c.ReadJSON(&msg); err != nil {
			break
		}

		// Menentukan aksi berdasarkan Tipe Pesan
		switch msg.Type {
		
		case "SONG_ENDED":
			// Datang dari Player (b860h) saat tag <audio> mencapai akhir
			ProcessNextSong()
			
		case "NEXT":
			// Datang dari tombol "Next" di Remote HP
			ProcessNextSong()

		case "PLAY_NOW":
			if idFloat, ok := msg.Payload.(float64); ok {
				songID := uint(idFloat)
				
				// 1. Matikan lagu yang sedang jalan
				database.DB.Model(&models.Queue{}).Where("status = ?", "playing").Update("status", "done")
				
				// 2. Ambil lagu spesifik yang ditekan user
				var targetSong models.Queue
				if err := database.DB.First(&targetSong, songID).Error; err == nil {
					// 3. Set jadi playing dan eksekusi
					targetSong.Status = "playing"
					database.DB.Save(&targetSong)
					
					log.Println("Perintah Force Play diterima dari Remote.")
					extractAndPlay(targetSong)
				}
			}

		case "PAUSE", "RESUME", "VOLUME", "SEEK":
			// Perintah kontrol standar.
			// Server tidak perlu memprosesnya, cukup "pantulkan" (broadcast) 
			// ke semua klien, nanti script di Player yang akan mengeksekusinya.
			broadcast(msg)
		}
	}
}