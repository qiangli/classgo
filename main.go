package main

import (
	"context"
	"database/sql"
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

	"classgo/internal/auth"
	"classgo/internal/backup"
	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/handlers"
	"classgo/internal/memos"
	"classgo/internal/models"

	"github.com/go-co-op/gocron/v2"

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

	// Auto-restore from latest backup only if DB appears to have lost data
	backupDir := filepath.Join(cfg.DataDir, "backups")
	var attendanceCount int
	db.QueryRow("SELECT COUNT(*) FROM attendance").Scan(&attendanceCount)
	if attendanceCount == 0 {
		if backupFile, restored, err := backup.Restore(backupDir, map[string]*sql.DB{"classgo": db}); err != nil {
			log.Printf("Warning: backup restore had errors: %v", err)
		} else if restored > 0 {
			log.Printf("Restored %d tables from %s", restored, filepath.Base(backupFile))
		}
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

	pinMode := cfg.PinMode
	if pinMode == "" {
		pinMode = "off"
	}

	app := &handlers.App{
		DB:          db,
		Tmpl:        tmpl,
		AppName:     cfg.AppName,
		DataDir:     cfg.DataDir,
		PinMode:     pinMode,
		MemosSyncer: memosSyncer,
		MemosStore:  memosStoreInst,
		Sessions:    auth.NewSessionStore(),
		RateLimiter: handlers.NewRateLimiter(),
	}
	app.SetRequirePIN(pinMode == "center")

	mux := http.NewServeMux()

	// Public routes — no authentication required
	mux.HandleFunc("/", handlers.NoCache(app.HandleMobile))
	mux.HandleFunc("/kiosk", handlers.NoCache(app.HandleKiosk))
	mux.HandleFunc("/login", handlers.NoCache(app.HandleLogin))
	mux.HandleFunc("/logout", handlers.NoCache(app.HandleLogout))
	mux.HandleFunc("/api/login", handlers.NoCache(app.HandleLoginAPI))
	mux.HandleFunc("/api/users/search", handlers.NoCache(app.HandleUserSearch))
	mux.HandleFunc("/api/settings", handlers.NoCache(app.HandleSettings))
	mux.HandleFunc("/api/students/search", handlers.NoCache(app.HandleStudentSearch))
	mux.HandleFunc("/api/checkin", handlers.NoCache(app.HandleCheckIn))
	mux.HandleFunc("/api/checkout", handlers.NoCache(app.HandleCheckOut))
	mux.HandleFunc("/api/status", handlers.NoCache(app.HandleStatus))
	mux.HandleFunc("/api/tracker/due", handlers.NoCache(app.HandleTrackerDue))
	mux.HandleFunc("/api/tracker/respond", handlers.NoCache(app.HandleTrackerRespond))
	mux.HandleFunc("/api/tracker/student-items", handlers.NoCache(app.HandleStudentTrackerItems))
	mux.HandleFunc("/api/tracker/student-items/delete", handlers.NoCache(app.HandleStudentTrackerItemDelete))
	mux.HandleFunc("/api/tracker/complete", handlers.NoCache(app.HandleTrackerComplete))
	mux.HandleFunc("/api/student/pin/setup", handlers.NoCache(app.HandleStudentPINSetup))

	// Dashboard — require any authenticated user
	mux.HandleFunc("/dashboard", handlers.NoCache(app.RequireAuth(app.HandleDashboard)))
	mux.HandleFunc("/api/dashboard/my-classes", handlers.NoCache(app.RequireAuth(app.HandleDashboardMyClasses)))
	mux.HandleFunc("/api/dashboard/my-students", handlers.NoCache(app.RequireAuth(app.HandleDashboardMyStudents)))
	mux.HandleFunc("/api/dashboard/all-tasks", handlers.NoCache(app.RequireAuth(app.HandleDashboardAllTasks)))
	mux.HandleFunc("/api/dashboard/teacher-items", handlers.NoCache(app.RequireAuth(app.HandleDashboardTeacherItems)))
	mux.HandleFunc("/api/dashboard/progress", handlers.NoCache(app.RequireAuth(app.HandleTrackerProgress)))
	mux.HandleFunc("/api/dashboard/bulk-assign", handlers.NoCache(app.RequireAuth(app.HandleTrackerBulkAssign)))

	// Admin pages — require admin role (redirect to login)
	mux.HandleFunc("/admin", handlers.NoCache(app.RequireAdmin(app.HandleAdmin)))
	mux.HandleFunc("/schedule", handlers.NoCache(app.RequireAdmin(app.HandleSchedulePage)))
	mux.HandleFunc("/admin/directory", handlers.NoCache(app.RequireAdmin(app.HandleDirectory)))
	mux.HandleFunc("/admin/export", handlers.NoCache(app.RequireAdmin(app.HandleExport)))
	mux.HandleFunc("/admin/export/xlsx", handlers.NoCache(app.RequireAdmin(app.HandleExportXLSX)))
	mux.HandleFunc("/admin/export/csv", handlers.NoCache(app.RequireAdmin(app.HandleExportCSV)))
	mux.HandleFunc("/admin/export/csv/zip", handlers.NoCache(app.RequireAdmin(app.HandleExportCSVZip)))

	// Admin APIs — require admin role (return 401/403 JSON)
	mux.HandleFunc("/api/admin/pin/toggle", handlers.NoCache(app.RequireAdminAPI(app.HandlePINToggle)))
	mux.HandleFunc("/api/admin/pin", handlers.NoCache(app.RequireAdminAPI(app.HandlePINChange)))
	mux.HandleFunc("/api/attendees", handlers.NoCache(app.RequireAdminAPI(app.HandleAttendees)))
	mux.HandleFunc("/api/attendees/metrics", handlers.NoCache(app.RequireAdminAPI(app.HandleAttendanceMetrics)))
	mux.HandleFunc("/api/v1/schedule/today", handlers.NoCache(app.RequireAdminAPI(app.HandleScheduleToday)))
	mux.HandleFunc("/api/v1/schedule/week", handlers.NoCache(app.RequireAdminAPI(app.HandleScheduleWeek)))
	mux.HandleFunc("/api/v1/schedule/conflicts", handlers.NoCache(app.RequireAdminAPI(app.HandleScheduleConflicts)))
	mux.HandleFunc("/api/v1/directory", handlers.NoCache(app.RequireAdminAPI(app.HandleDirectoryAPI)))
	mux.HandleFunc("/api/v1/import", handlers.NoCache(app.RequireAdminAPI(app.HandleImportData)))
	mux.HandleFunc("/api/v1/data", handlers.NoCache(app.RequireAdminAPI(app.HandleDataCRUD)))
	mux.HandleFunc("/api/v1/password-reset", handlers.NoCache(app.RequireAdminAPI(app.HandlePasswordReset)))
	mux.HandleFunc("/api/v1/memos/sync", handlers.NoCache(app.RequireAdminAPI(app.HandleMemosSync)))
	mux.HandleFunc("/api/v1/tracker/items", handlers.NoCache(app.RequireAdminAPI(app.HandleTrackerItems)))
	mux.HandleFunc("/api/v1/tracker/items/delete", handlers.NoCache(app.RequireAdminAPI(app.HandleTrackerItemDelete)))
	mux.HandleFunc("/api/v1/tracker/responses", handlers.NoCache(app.RequireAdminAPI(app.HandleTrackerResponses)))
	mux.HandleFunc("/api/admin/pin/mode", handlers.NoCache(app.RequireAdminAPI(app.HandlePINModeChange)))
	mux.HandleFunc("/api/v1/student/pin/reset", handlers.NoCache(app.RequireAdminAPI(app.HandleStudentPINReset)))
	mux.HandleFunc("/api/v1/student/pin/require", handlers.NoCache(app.RequireAdminAPI(app.HandleStudentRequirePIN)))
	mux.HandleFunc("/api/v1/audit/flags", handlers.NoCache(app.RequireAdminAPI(app.HandleAuditFlags)))
	mux.HandleFunc("/api/v1/audit/devices", handlers.NoCache(app.RequireAdminAPI(app.HandleAuditDevices)))
	mux.HandleFunc("/api/v1/audit/dismiss", handlers.NoCache(app.RequireAdminAPI(app.HandleAuditDismiss)))

	// Memos — require any authenticated user (redirect to login)
	mux.Handle("/memos/", handlers.NoCache(app.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/memos", memosServer.Handler()).ServeHTTP(w, r)
	})))

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

	// Set up daily backup at midnight via gocron
	backupDBs := func() map[string]*sql.DB {
		dbs := map[string]*sql.DB{"classgo": db}
		// Open a read-only connection to memos DB for backup
		memosDB, err := database.OpenDB(memosDSN)
		if err == nil {
			dbs["memos"] = memosDB
		}
		return dbs
	}

	scheduler, err := gocron.NewScheduler()
	if err != nil {
		log.Printf("Warning: backup scheduler not started: %v", err)
	} else {
		_, err = scheduler.NewJob(
			gocron.CronJob("0 0 * * *", false), // midnight daily
			gocron.NewTask(func() {
				dbs := backupDBs()
				backup.Run(backupDir, dbs)
				// Close the temporary memos DB connection
				if memosDB, ok := dbs["memos"]; ok {
					memosDB.Close()
				}
			}),
		)
		if err != nil {
			log.Printf("Warning: failed to schedule backup: %v", err)
		} else {
			scheduler.Start()
			log.Printf("  Backup:  daily at midnight → %s", backupDir)
		}
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")

		// Shutdown backup on exit
		dbs := backupDBs()
		backup.RunOnShutdown(backupDir, dbs)
		if memosDB, ok := dbs["memos"]; ok {
			memosDB.Close()
		}

		if scheduler != nil {
			scheduler.Shutdown()
		}
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
