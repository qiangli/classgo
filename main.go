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
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	"classgo/internal/auth"
	"classgo/internal/backup"
	"classgo/internal/cloudsync"
	"classgo/internal/database"
	"classgo/internal/datastore"
	"classgo/internal/handlers"
	"classgo/internal/memos"
	"classgo/internal/models"
	"classgo/internal/reports"
	"classgo/internal/scheduler"
	"classgo/internal/tunnel"

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
	flagPort := flag.Int("port", 0, "Server port (default 8080)")
	flagDB := flag.String("db", "", "Database file path (default ./classgo.db)")
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

	if *flagPort > 0 {
		cfg.Port = *flagPort
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	if *flagDB != "" {
		cfg.DBPath = *flagDB
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "./classgo.db"
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

	db, err := database.OpenDB(cfg.DBPath)
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
	database.SeedSampleData(db)

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

	processUser := ""
	if u, err := user.Current(); err == nil {
		processUser = u.Username
	}

	app := &handlers.App{
		DB:             db,
		Tmpl:           tmpl,
		AppName:        cfg.AppName,
		DataDir:        cfg.DataDir,
		PinMode:        pinMode,
		MemosSyncer:    memosSyncer,
		MemosStore:     memosStoreInst,
		Sessions:       auth.NewSessionStore(),
		RateLimiter:    handlers.NewRateLimiter(),
		Administrators: cfg.Administrators,
		ProcessUser:    processUser,
		CloudSync:      cfg.CloudSync,
	}
	app.SetRequirePIN(pinMode == "center")

	mux := http.NewServeMux()

	// Public routes — no authentication required
	mux.HandleFunc("/", handlers.NoCache(app.HandleMobile))
	mux.HandleFunc("/home", handlers.NoCache(app.HandleHome))
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
	mux.HandleFunc("/api/tracker/late-signoff", handlers.NoCache(app.RequireAuth(app.HandleLateSignoff)))
	mux.HandleFunc("/api/student/pin/setup", handlers.NoCache(app.HandleStudentPINSetup))
	mux.HandleFunc("/api/pin/check", handlers.NoCache(app.HandlePINCheck))

	// Dashboard — require any authenticated user
	mux.HandleFunc("/dashboard", handlers.NoCache(app.RequireAuth(app.HandleDashboard)))
	mux.HandleFunc("/api/dashboard/my-classes", handlers.NoCache(app.RequireAuth(app.HandleDashboardMyClasses)))
	mux.HandleFunc("/api/dashboard/my-students", handlers.NoCache(app.RequireAuth(app.HandleDashboardMyStudents)))
	mux.HandleFunc("/api/dashboard/all-tasks", handlers.NoCache(app.RequireAuth(app.HandleDashboardAllTasks)))
	mux.HandleFunc("/api/dashboard/teacher-items", handlers.NoCache(app.RequireAuth(app.HandleDashboardTeacherItems)))
	mux.HandleFunc("/api/dashboard/progress", handlers.NoCache(app.RequireAuth(app.HandleTrackerProgress)))
	mux.HandleFunc("/api/dashboard/bulk-assign", handlers.NoCache(app.RequireAuth(app.HandleTrackerBulkAssign)))
	mux.HandleFunc("/api/dashboard/assign-library-item", handlers.NoCache(app.RequireAuth(app.HandleAssignLibraryItem)))
	mux.HandleFunc("/api/dashboard/classes", handlers.NoCache(app.RequireAuth(app.HandleDashboardClasses)))
	mux.HandleFunc("/api/dashboard/enroll", handlers.NoCache(app.RequireAuth(app.HandleDashboardEnroll)))
	mux.HandleFunc("/api/dashboard/unenroll", handlers.NoCache(app.RequireAuth(app.HandleDashboardUnenroll)))
	mux.HandleFunc("/api/dashboard/schedule", handlers.NoCache(app.RequireAuth(app.HandleDashboardScheduleSave)))
	mux.HandleFunc("/api/dashboard/schedule/delete", handlers.NoCache(app.RequireAuth(app.HandleDashboardScheduleDelete)))
	mux.HandleFunc("/api/dashboard/rooms", handlers.NoCache(app.RequireAuth(app.HandleDashboardRooms)))

	// User profile — require any authenticated user
	mux.HandleFunc("/profile", handlers.NoCache(app.RequireAuth(app.HandleUserProfilePage)))
	mux.HandleFunc("/api/v1/user/profile", handlers.NoCache(app.RequireAuth(app.HandleUserProfile)))

	// Admin login — dedicated page (no auth required)
	mux.HandleFunc("/admin/login", handlers.NoCache(app.HandleAdminLogin))
	mux.HandleFunc("/admin/api/login", handlers.NoCache(app.HandleAdminLoginAPI))

	// Admin pages — require admin role (redirect to admin login)
	mux.HandleFunc("/admin", handlers.NoCache(app.RequireAdmin(app.HandleAdmin)))
	mux.HandleFunc("/admin/schedule", handlers.NoCache(app.RequireAdmin(app.HandleSchedulePage)))
	mux.HandleFunc("/admin/directory", handlers.NoCache(app.RequireAdmin(app.HandleDirectory)))
	mux.HandleFunc("/admin/profile", handlers.NoCache(app.RequireAdmin(app.HandleProfilePage)))
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
	mux.HandleFunc("/api/v1/tracker/field-values", handlers.NoCache(app.RequireAdminAPI(app.HandleStudentFieldValues)))
	mux.HandleFunc("/api/v1/admin/progress-summary", handlers.NoCache(app.RequireAdminAPI(app.HandleAdminProgressSummary)))
	mux.HandleFunc("/api/v1/tracker/items/delete", handlers.NoCache(app.RequireAdminAPI(app.HandleTrackerItemDelete)))
	mux.HandleFunc("/api/v1/tracker/responses", handlers.NoCache(app.RequireAdminAPI(app.HandleTrackerResponses)))
	mux.HandleFunc("/api/admin/pin/mode", handlers.NoCache(app.RequireAdminAPI(app.HandlePINModeChange)))
	mux.HandleFunc("/api/v1/student/pin/reset", handlers.NoCache(app.RequireAdminAPI(app.HandleStudentPINReset)))
	mux.HandleFunc("/api/v1/student/pin/require", handlers.NoCache(app.RequireAdminAPI(app.HandleStudentRequirePIN)))
	mux.HandleFunc("/api/v1/audit/flags", handlers.NoCache(app.RequireAdminAPI(app.HandleAuditFlags)))
	mux.HandleFunc("/api/v1/audit/devices", handlers.NoCache(app.RequireAdminAPI(app.HandleAuditDevices)))
	mux.HandleFunc("/api/v1/audit/dismiss", handlers.NoCache(app.RequireAdminAPI(app.HandleAuditDismiss)))
	mux.HandleFunc("/api/v1/student/profile", handlers.NoCache(app.RequireAdminAPI(app.HandleStudentProfile)))
	mux.HandleFunc("/api/v1/preferences", handlers.NoCache(app.RequireAuth(app.HandlePreferences)))

	// Reports — require any authenticated user (role-filtered)
	mux.HandleFunc("/reports", handlers.NoCache(app.RequireAuth(app.HandleReportsPage)))
	mux.HandleFunc("/api/v1/reports/catalog", handlers.NoCache(app.RequireAuth(app.HandleReportCatalog)))
	mux.HandleFunc("/api/v1/reports/data", handlers.NoCache(app.RequireAuth(app.HandleReportAPI)))
	mux.HandleFunc("/api/v1/reports/subscriptions", handlers.NoCache(app.RequireAuth(app.HandleReportSubscriptions)))

	// Memos — require any authenticated user (redirect to login)
	mux.Handle("/memos/", handlers.NoCache(app.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/memos", memosServer.Handler()).ServeHTTP(w, r)
	})))

	// Proxy Memos static assets referenced without /memos/ prefix (hardcoded in Memos React SPA)
	for _, asset := range []string{"/logo.webp", "/full-logo.webp", "/site.webmanifest", "/apple-touch-icon.png"} {
		path := asset
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			memosServer.Handler().ServeHTTP(w, r)
		})
	}

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	ipURL := fmt.Sprintf("http://%s:%d", handlers.GetLocalIP(), cfg.Port)
	mdnsURL := fmt.Sprintf("http://%s:%d", handlers.GetMDNSHostname(), cfg.Port)
	pin := app.EnsureDailyPIN()

	log.Println("=================================")
	log.Printf("  %s Attendance Server", cfg.AppName)
	log.Println("=================================")
	log.Printf("  Server:  %s", mdnsURL)
	log.Printf("           %s", ipURL)
	log.Printf("  Admin:   %s/admin", mdnsURL)
	log.Printf("  Kiosk:   %s/kiosk", mdnsURL)
	log.Printf("  Home:    %s/home", mdnsURL)
	log.Printf("  PIN:     %s", pin)
	log.Printf("  Data:    %s", cfg.DataDir)
	if cfg.Tunnel.Enabled {
		log.Printf("  Tunnel:  %s", tunnel.PublicURL(cfg.Tunnel))
	}
	log.Println("=================================")

	var handler http.Handler = mux
	if cfg.Tunnel.Enabled {
		handler = handlers.TunnelGuard(handler, cfg.Tunnel.AllowedRoutes)
	}
	handler = handlers.AllowPrivateNetwork(handler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: handler,
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

	sched, err := gocron.NewScheduler(
		gocron.WithLocation(time.Local), // explicit local timezone — DST-safe
	)
	if err != nil {
		log.Printf("Warning: backup scheduler not started: %v", err)
	} else {
		_, err = sched.NewJob(
			gocron.CronJob("0 22 * * *", false), // 10:00 PM daily
			gocron.NewTask(func() {
				dbs := backupDBs()
				backup.Run(backupDir, dbs)
				// Close the temporary memos DB connection
				if memosDB, ok := dbs["memos"]; ok {
					memosDB.Close()
				}
			}),
			gocron.WithName("daily-backup"),
		)
		if err != nil {
			log.Printf("Warning: failed to schedule backup: %v", err)
		}

		_, err = sched.NewJob(
			gocron.CronJob("0 21 * * *", false), // 9:00 PM daily
			gocron.NewTask(func() {
				if err := datastore.ExportDailyAttendanceXLSX(db, cfg.DataDir); err != nil {
					log.Printf("Daily attendance export failed: %v", err)
				} else {
					log.Printf("Daily attendance exported to %s", cfg.DataDir)
				}
			}),
			gocron.WithName("daily-attendance-export"),
		)
		if err != nil {
			log.Printf("Warning: failed to schedule attendance export: %v", err)
		}

		if cfg.CloudSync.Enabled {
			syncSchedule := cfg.CloudSync.Schedule
			if syncSchedule == "" {
				syncSchedule = "30 22 * * *" // 10:30 PM daily (after backup)
			}
			// Generate rclone config at startup so admin can verify
			if confPath, err := cloudsync.GenerateRcloneConf(cfg.CloudSync, cfg.DataDir); err != nil {
				log.Printf("Warning: failed to generate rclone config: %v", err)
			} else {
				log.Printf("  Rclone config: %s", confPath)
			}
			syncCfg := cfg.CloudSync
			syncDataDir := cfg.DataDir
			_, err = sched.NewJob(
				gocron.CronJob(syncSchedule, false),
				gocron.NewTask(func() {
					if err := cloudsync.Run(syncCfg, syncDataDir); err != nil {
						log.Printf("Cloud sync failed: %v", err)
					}
				}),
				gocron.WithName("cloud-sync"),
			)
			if err != nil {
				log.Printf("Warning: failed to schedule cloud sync: %v", err)
			}
		}

		// Report cron jobs
		reportDB := db
		reportDir := cfg.DataDir
		_, err = sched.NewJob(
			gocron.CronJob("30 21 * * *", false), // 9:30 PM daily
			gocron.NewTask(func() {
				reports.RunDailyAttendance(reportDB, reportDir)
			}),
			gocron.WithName("report-daily-attendance"),
		)
		if err != nil {
			log.Printf("Warning: failed to schedule daily attendance report: %v", err)
		}

		_, err = sched.NewJob(
			gocron.CronJob("0 21 * * 0", false), // Sunday 9 PM
			gocron.NewTask(func() {
				reports.RunWeeklyAudit(reportDB, reportDir)
			}),
			gocron.WithName("report-weekly-audit"),
		)
		if err != nil {
			log.Printf("Warning: failed to schedule weekly audit report: %v", err)
		}

		_, err = sched.NewJob(
			gocron.CronJob("0 7 1 * *", false), // 1st of month 7 AM
			gocron.NewTask(func() {
				reports.RunMonthlyDashboard(reportDB, reportDir)
			}),
			gocron.WithName("report-monthly-dashboard"),
		)
		if err != nil {
			log.Printf("Warning: failed to schedule monthly dashboard report: %v", err)
		}

		_, err = sched.NewJob(
			gocron.CronJob("0 18 * * *", false), // 6 PM daily — process subscriptions
			gocron.NewTask(func() {
				reports.ProcessSubscriptions(reportDB, reportDir)
			}),
			gocron.WithName("report-subscriptions"),
		)
		if err != nil {
			log.Printf("Warning: failed to schedule report subscriptions: %v", err)
		}

		sched.Start()
		log.Printf("  Backup:  daily at 10:00 PM → %s", backupDir)
		log.Printf("  Export:  daily at 9:00 PM → %s/attendances/attendance-*.xlsx", cfg.DataDir)
		log.Printf("  Reports: daily/weekly/monthly → %s/reports/", cfg.DataDir)
		if cfg.CloudSync.Enabled {
			schedule := cfg.CloudSync.Schedule
			if schedule == "" {
				schedule = "30 22 * * *"
			}
			log.Printf("  Sync:    %s → cloud (%s)", schedule, cfg.CloudSync.Provider)
		}

		schedulerUI := scheduler.NewHandler(sched)
		schedulerUI.RegisterRoutes(mux, func(next http.HandlerFunc) http.HandlerFunc {
			return handlers.NoCache(app.RequireSuperAdminAPI(next))
		})
	}

	// Start tunnel if enabled
	var tunnelCmd *exec.Cmd
	if cfg.Tunnel.Enabled {
		var err error
		tunnelCmd, err = tunnel.Start(cfg.Tunnel, cfg.DataDir)
		if err != nil {
			log.Printf("Warning: tunnel not started: %v", err)
		}
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")

		tunnel.Stop(tunnelCmd)

		// Shutdown backup on exit
		dbs := backupDBs()
		backup.RunOnShutdown(backupDir, dbs)
		if memosDB, ok := dbs["memos"]; ok {
			memosDB.Close()
		}

		if sched != nil {
			sched.Shutdown()
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
