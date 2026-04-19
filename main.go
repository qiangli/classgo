package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/handlers"
	"classgo/internal/models"
)

func loadConfig() models.Config {
	cfg := models.Config{
		AppName: "LERN",
		DataDir: "./data",
	}

	if data, err := os.ReadFile("config.json"); err == nil {
		json.Unmarshal(data, &cfg)
	}

	if env := os.Getenv("APP_NAME"); env != "" {
		cfg.AppName = env
	}

	flagName := flag.String("name", "", "Application name")
	flagDataDir := flag.String("data-dir", "", "Data directory (default ./data)")
	flagRebuild := flag.Bool("rebuild-db", false, "Drop and rebuild index tables from spreadsheet files")
	flag.Parse()

	if *flagName != "" {
		cfg.AppName = *flagName
	}
	if *flagDataDir != "" {
		cfg.DataDir = *flagDataDir
	}

	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}

	// Store rebuild flag in a package-level var for use below
	rebuildDB = *flagRebuild

	return cfg
}

var rebuildDB bool

func main() {
	cfg := loadConfig()

	db, err := database.OpenDB("./classgo.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	if rebuildDB {
		log.Println("Rebuilding index tables...")
		if err := database.DropIndexTables(db); err != nil {
			log.Fatal("Failed to drop index tables:", err)
		}
	}

	if err := database.MigrateDB(db); err != nil {
		log.Fatal("Failed to initialize database schema:", err)
	}

	// Import spreadsheet data
	entityData, err := datastore.ReadAll(cfg.DataDir)
	if err != nil {
		log.Printf("Warning: could not read data files: %v", err)
	} else if len(entityData.Students)+len(entityData.Parents)+len(entityData.Teachers)+len(entityData.Rooms)+len(entityData.Schedules) > 0 {
		if err := datastore.ImportAll(db, entityData); err != nil {
			log.Printf("Warning: import failed: %v", err)
		}
	}

	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Failed to parse templates:", err)
	}

	app := &handlers.App{
		DB:      db,
		Tmpl:    tmpl,
		AppName: cfg.AppName,
		DataDir: cfg.DataDir,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlers.NoCache(app.HandleMobile))
	mux.HandleFunc("/kiosk", handlers.NoCache(app.HandleKiosk))
	mux.HandleFunc("/admin", handlers.NoCache(app.HandleAdmin))
	mux.HandleFunc("/api/signin", handlers.NoCache(app.HandleSignIn))
	mux.HandleFunc("/api/signout", handlers.NoCache(app.HandleSignOut))
	mux.HandleFunc("/api/status", handlers.NoCache(app.HandleStatus))
	mux.HandleFunc("/api/attendees", handlers.NoCache(app.HandleAttendees))
	mux.HandleFunc("/admin/export", handlers.NoCache(app.HandleExport))
	mux.HandleFunc("/admin/export/xlsx", handlers.NoCache(app.HandleExportXLSX))
	mux.HandleFunc("/schedule", handlers.NoCache(app.HandleSchedulePage))
	mux.HandleFunc("/api/v1/schedule/today", handlers.NoCache(app.HandleScheduleToday))
	mux.HandleFunc("/api/v1/schedule/week", handlers.NoCache(app.HandleScheduleWeek))
	mux.HandleFunc("/api/v1/schedule/conflicts", handlers.NoCache(app.HandleScheduleConflicts))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	ipURL := fmt.Sprintf("http://%s:8080", handlers.GetLocalIP())
	mdnsURL := fmt.Sprintf("http://%s:8080", handlers.GetMDNSHostname())
	pin := app.EnsureDailyPIN()

	log.Println("=================================")
	log.Printf("  %s Attendance Server", cfg.AppName)
	log.Println("=================================")
	log.Printf("  Server:  %s", mdnsURL)
	log.Printf("           %s", ipURL)
	log.Printf("  Admin:   %s/admin", mdnsURL)
	log.Printf("  Kiosk:   %s/kiosk", mdnsURL)
	log.Printf("  PIN:     %s", pin)
	log.Printf("  Data:    %s", cfg.DataDir)
	log.Println("=================================")

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start file watcher for live reimport
	watcher, err := datastore.NewWatcher(cfg.DataDir, db, nil)
	if err != nil {
		log.Printf("Warning: file watcher not started: %v", err)
	} else {
		watcher.Start()
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		if watcher != nil {
			watcher.Stop()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		db.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal("Server error:", err)
	}
}
