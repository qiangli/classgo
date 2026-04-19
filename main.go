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
	"path/filepath"
	"syscall"
	"time"

	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/handlers"
	"classgo/internal/memos"
	"classgo/internal/models"

	memosprofile "classgo/memos/lib/profile"
	memosserver "classgo/memos/server"
	memosstore "classgo/memos/store"
	memossqlite "classgo/memos/store/db/sqlite"
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

	rebuildDB = *flagRebuild

	return cfg
}

var rebuildDB bool

func main() {
	cfg := loadConfig()
	ctx := context.Background()

	// Ensure data directory exists
	os.MkdirAll(cfg.DataDir, 0755)

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

	// Initialize embedded Memos server
	memosDataDir := filepath.Join(cfg.DataDir, "memos")
	os.MkdirAll(memosDataDir, 0755)
	memosDSN := filepath.Join(memosDataDir, "memos_prod.db")

	memosProfile := &memosprofile.Profile{
		Data:    memosDataDir,
		DSN:     memosDSN,
		Driver:  "sqlite",
		Version: "0.27.1",
	}
	if err := memosProfile.Validate(); err != nil {
		log.Fatalf("Memos profile validation failed: %v", err)
	}

	memosDriver, err := memossqlite.NewDB(memosProfile)
	if err != nil {
		log.Fatalf("Failed to open Memos database: %v", err)
	}

	memosStoreInst := memosstore.New(memosDriver, memosProfile)
	if err := memosStoreInst.Migrate(ctx); err != nil {
		log.Fatalf("Failed to migrate Memos database: %v", err)
	}

	memosServer, err := memosserver.NewServer(ctx, memosProfile, memosStoreInst)
	if err != nil {
		log.Fatalf("Failed to create Memos server: %v", err)
	}

	// Initialize Memos syncer with direct store access
	adminUserID, err := memos.EnsureAdminUser(memosStoreInst, "tutoros")
	if err != nil {
		log.Printf("Warning: could not ensure Memos admin user: %v", err)
	}
	memosClient := memos.NewClient(memosStoreInst, adminUserID)
	memosSyncer := memos.NewSyncer(memosClient, db)

	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Failed to parse templates:", err)
	}

	app := &handlers.App{
		DB:          db,
		Tmpl:        tmpl,
		AppName:     cfg.AppName,
		DataDir:     cfg.DataDir,
		MemosSyncer: memosSyncer,
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
	mux.HandleFunc("/api/v1/memos/sync", handlers.NoCache(app.HandleMemosSync))

	// Mount Memos under /memos/ — StripPrefix removes /memos before Echo sees the request
	mux.Handle("/memos/", http.StripPrefix("/memos", memosServer.Handler()))

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
	log.Printf("  Memos:   %s/memos/", mdnsURL)
	log.Printf("  PIN:     %s", pin)
	log.Printf("  Data:    %s", cfg.DataDir)
	log.Println("=================================")

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start file watcher
	watcherCallback := func() {
		if memosSyncer != nil {
			if err := memosSyncer.SyncAll(); err != nil {
				log.Printf("Memos sync after file change: %v", err)
			}
		}
	}
	watcher, err := datastore.NewWatcher(cfg.DataDir, db, watcherCallback)
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
		memosServer.Shutdown(ctx)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		db.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal("Server error:", err)
	}
}
