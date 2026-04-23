package scheduler

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFiles embed.FS

// JobData represents the job information sent to clients.
type JobData struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Tags           []string `json:"tags"`
	NextRun        string   `json:"nextRun"`
	LastRun        string   `json:"lastRun"`
	NextRuns       []string `json:"nextRuns"`
	Schedule       string   `json:"schedule"`
	ScheduleDetail string   `json:"scheduleDetail"`
	SchedulerName  string   `json:"schedulerName"`
}

// Handler manages the scheduler UI API and WebSocket connections.
type Handler struct {
	schedulers     []gocron.Scheduler
	schedulerNames []string
	wsClients      map[*websocket.Conn]bool
	wsMutex        sync.RWMutex
	upgrader       websocket.Upgrader
	done           chan struct{}
}

// NewHandler creates a new scheduler UI handler for the given scheduler.
func NewHandler(s gocron.Scheduler) *Handler {
	h := &Handler{
		schedulers:     []gocron.Scheduler{s},
		schedulerNames: []string{"Default"},
		wsClients:      make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		done: make(chan struct{}),
	}
	go h.broadcastJobUpdates()
	return h
}

// RegisterRoutes registers all scheduler UI routes on the given mux with auth middleware.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, auth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("/api/v1/scheduler/jobs", auth(h.handleJobs))
	mux.HandleFunc("/api/v1/scheduler/jobs/", auth(h.handleJobByID))
	mux.HandleFunc("/api/v1/scheduler/status", auth(h.handleStatus))
	mux.HandleFunc("/api/v1/scheduler/ws", auth(h.handleWebSocket))

	sub, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/static/scheduler/", http.StripPrefix("/static/scheduler/", http.FileServer(http.FS(sub))))
}

// Shutdown stops the broadcast goroutine.
func (h *Handler) Shutdown() {
	close(h.done)
}

// handleJobs returns all jobs across all schedulers.
func (h *Handler) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	respondJSON(w, http.StatusOK, h.getJobsData())
}

// handleJobByID handles GET /api/v1/scheduler/jobs/{id} and POST /api/v1/scheduler/jobs/{id}/run.
func (h *Handler) handleJobByID(w http.ResponseWriter, r *http.Request) {
	// Path: /api/v1/scheduler/jobs/{id}[/run]
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/scheduler/jobs/")
	parts := strings.SplitN(path, "/", 2)
	idStr := parts[0]

	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	// POST .../run
	if len(parts) == 2 && parts[1] == "run" && r.Method == http.MethodPost {
		h.runJob(w, id)
		return
	}

	// GET single job
	if r.Method == http.MethodGet {
		h.getJob(w, id)
		return
	}

	respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

func (h *Handler) getJob(w http.ResponseWriter, id uuid.UUID) {
	for i, sched := range h.schedulers {
		for _, job := range sched.Jobs() {
			if job.ID() == id {
				respondJSON(w, http.StatusOK, h.convertJobToData(job, h.schedulerNames[i]))
				return
			}
		}
	}
	respondError(w, http.StatusNotFound, "Job not found")
}

func (h *Handler) runJob(w http.ResponseWriter, id uuid.UUID) {
	for _, sched := range h.schedulers {
		for _, job := range sched.Jobs() {
			if job.ID() == id {
				if err := job.RunNow(); err != nil {
					respondError(w, http.StatusInternalServerError, err.Error())
					return
				}
				respondJSON(w, http.StatusOK, map[string]string{"message": "Job executed"})
				return
			}
		}
	}
	respondError(w, http.StatusNotFound, "Job not found")
}

// handleStatus returns scheduler status.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	total := 0
	for _, sched := range h.schedulers {
		total += len(sched.Jobs())
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"schedulers": len(h.schedulers),
		"totalJobs":  total,
	})
}

// handleWebSocket upgrades to WebSocket and streams job updates.
func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	h.wsMutex.Lock()
	h.wsClients[conn] = true
	h.wsMutex.Unlock()

	// Send initial job list.
	jobs := h.getJobsData()
	_ = conn.WriteJSON(map[string]interface{}{
		"type": "jobs",
		"data": jobs,
	})

	// Keep connection alive; exit on read error (client disconnect).
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			h.wsMutex.Lock()
			delete(h.wsClients, conn)
			h.wsMutex.Unlock()
			break
		}
	}
}

// broadcastJobUpdates sends job state to all WebSocket clients every second.
func (h *Handler) broadcastJobUpdates() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.done:
			return
		case <-ticker.C:
		}

		h.wsMutex.RLock()
		if len(h.wsClients) == 0 {
			h.wsMutex.RUnlock()
			continue
		}
		h.wsMutex.RUnlock()

		jobs := h.getJobsData()
		message := map[string]interface{}{
			"type": "jobs",
			"data": jobs,
		}

		h.wsMutex.Lock()
		for client := range h.wsClients {
			if err := client.WriteJSON(message); err != nil {
				client.Close()
				delete(h.wsClients, client)
			}
		}
		h.wsMutex.Unlock()
	}
}

// getJobsData collects job data from all schedulers.
func (h *Handler) getJobsData() []JobData {
	var result []JobData
	for i, sched := range h.schedulers {
		for _, job := range sched.Jobs() {
			result = append(result, h.convertJobToData(job, h.schedulerNames[i]))
		}
	}
	return result
}

func (h *Handler) convertJobToData(job gocron.Job, schedulerName string) JobData {
	nextRun, _ := job.NextRun()
	lastRun, _ := job.LastRunStartedAt()
	nextRuns, _ := job.NextRuns(5)

	schedule, scheduleDetail := inferSchedule(job, nextRuns)

	return JobData{
		ID:             job.ID().String(),
		Name:           job.Name(),
		Tags:           job.Tags(),
		NextRun:        formatTime(nextRun),
		LastRun:        formatTime(lastRun),
		NextRuns:       formatTimes(nextRuns),
		Schedule:       schedule,
		ScheduleDetail: scheduleDetail,
		SchedulerName:  schedulerName,
	}
}

func inferSchedule(job gocron.Job, nextRuns []time.Time) (string, string) {
	// Try to infer from next runs interval.
	if len(nextRuns) >= 2 {
		interval := nextRuns[1].Sub(nextRuns[0])

		if interval < time.Minute {
			seconds := int(interval.Seconds())
			return fmt.Sprintf("Every %d seconds", seconds), fmt.Sprintf("Duration: %ds", seconds)
		}
		if interval < time.Hour {
			minutes := int(interval.Minutes())
			return fmt.Sprintf("Every %d minutes", minutes), fmt.Sprintf("Duration: %dm", minutes)
		}
		if interval < 24*time.Hour {
			hours := int(interval.Hours())
			return fmt.Sprintf("Every %d hours", hours), fmt.Sprintf("Duration: %dh", hours)
		}
		days := int(interval.Hours() / 24)
		return fmt.Sprintf("Every %d days", days), fmt.Sprintf("Duration: %dd", days)
	}

	return "Scheduled", "Custom schedule"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatTimes(times []time.Time) []string {
	result := make([]string, 0, len(times))
	for _, t := range times {
		result = append(result, formatTime(t))
	}
	return result
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
