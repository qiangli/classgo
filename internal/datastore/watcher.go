package datastore

import (
	"database/sql"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches the data directory for spreadsheet changes and re-imports.
type Watcher struct {
	dataDir string
	db      *sql.DB
	watcher *fsnotify.Watcher
	done    chan struct{}

	mu       sync.Mutex
	onChange func() // optional callback after successful import
}

// NewWatcher creates a file watcher for the data directory.
func NewWatcher(dataDir string, db *sql.DB, onChange func()) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		dataDir:  dataDir,
		db:       db,
		watcher:  fsw,
		done:     make(chan struct{}),
		onChange: onChange,
	}

	// Watch the data dir itself (for tutoros.xlsx)
	if err := fsw.Add(dataDir); err != nil {
		log.Printf("Watcher: cannot watch %s: %v", dataDir, err)
	}

	// Watch csv subdirectory if it exists
	csvDir := filepath.Join(dataDir, "csv")
	if err := fsw.Add(csvDir); err != nil {
		log.Printf("Watcher: cannot watch %s: %v (will use xlsx only)", csvDir, err)
	}

	return w, nil
}

// Start begins watching in a background goroutine. Call Stop() to end.
func (w *Watcher) Start() {
	go w.loop()
	log.Printf("Watcher: monitoring %s for changes", w.dataDir)
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.done)
	w.watcher.Close()
}

func (w *Watcher) loop() {
	// Debounce: collect events for 500ms before triggering import
	var timer *time.Timer

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if !isRelevantFile(event.Name) {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Reset debounce timer
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(500*time.Millisecond, func() {
				w.reimport()
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)

		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

func (w *Watcher) reimport() {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := ReadAll(w.dataDir)
	if err != nil {
		log.Printf("Watcher: read error: %v", err)
		return
	}

	total := len(data.Students) + len(data.Parents) + len(data.Teachers) + len(data.Rooms) + len(data.Schedules)
	if total == 0 {
		return
	}

	if err := ImportAll(w.db, data); err != nil {
		log.Printf("Watcher: import error: %v", err)
		return
	}

	log.Println("Watcher: reimported data after file change")
	if w.onChange != nil {
		w.onChange()
	}
}

func isRelevantFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	if ext == ".xlsx" && strings.Contains(base, "tutoros") {
		return true
	}
	if ext == ".csv" {
		return true
	}
	return false
}
