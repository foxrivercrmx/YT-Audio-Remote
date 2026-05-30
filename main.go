package main

import (
	"embed"
	"flag"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/websocket/v2"

	// Sesuaikan "yt-audio-remote" dengan nama module di go.mod kamu nanti
	"yt-audio-remote/database"
	"yt-audio-remote/handlers"
)

// Menyematkan folder public langsung ke dalam binary agar portabel
//go:embed public/*
var publicFS embed.FS

func main() {
	dbPath := flag.String("db", "yt-audio.db", "Path ke file database SQLite")
	port := flag.String("port", "5520", "Port server jalan")
	cookiesPath := flag.String("cookies", "", "Path ke file cookies.txt untuk yt-dlp (opsional)")
	jsRuntime := flag.String("js-runtimes", "", "Pilihan JS runtime untuk yt-dlp, misal: quickjs (opsional)")
	flag.Parse() // Wajib dipanggil buat ngebaca inputanny

	// 1. Inisialisasi Database SQLite
	database.ConnectDB(*dbPath)

	handlers.YTCookiesFile = *cookiesPath
	handlers.YTJSRuntime = *jsRuntime

	// 2. Inisialisasi Fiber App
	app := fiber.New(fiber.Config{
		AppName:           "YT Audio Remote",
		DisableKeepalive:  false,
		// Trust proxy agar IP asli terbaca jika di belakang Caddy
		EnableTrustedProxyCheck: false, 
	})

	// 3. Middleware Global
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Nanti bisa dikunci ke domain spesifikmu
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// 4. Middleware WebSocket Upgrade
	// Memastikan request yang masuk ke /ws adalah benar-benar minta upgrade ke websocket
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// 6. Grup Routing API untuk Remote
	api := app.Group("/api")

	// Endpoint untuk Pengaturan Audio
	api.Get("/config", handlers.GetConfig)
	api.Post("/config", handlers.UpdateConfig)
	
	// Endpoint untuk memproses input dari form pencarian/URL
	api.Get("/search", handlers.SearchYouTube)       // Mode pencarian teks biasa
	api.Post("/parse", handlers.ParseURL)            // Mode parsing URL direct (music.youtube.com, Playlist, dsb)
	
	// Endpoint untuk manajemen antrian
	api.Get("/queue", handlers.GetQueue)             // Mengambil status dan list antrian saat ini
	api.Post("/queue", handlers.AddQueue)
	api.Delete("/queue/clear", handlers.ClearQueue)
	api.Delete("/queue/:id", handlers.DeleteQueue)   // Menghapus lagu dari antrian
	api.Post("/queue/reorder", handlers.ReorderQueue)// Opsional: Untuk fitur geser urutan lagu di remote

	// 7. Rute Utama WebSocket (Hubungan Real-time Player STB & Remote HP)
	app.Get("/ws", websocket.New(handlers.WebSocketHub))

	// 5. Rute File Statis Frontend (Remote UI & Player UI)
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(publicFS),
		PathPrefix: "public",
		Browse:     false,
		Index:      "index.html", // Default ke remote, player diakses via /player.html
	}))

	// 8. Jalankan Server
	log.Printf("Backend YT Audio berjalan di port: %s", *port)
	log.Fatal(app.Listen(":" + *port))
}