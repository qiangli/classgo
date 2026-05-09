package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"classgo/internal/datastore"

	"github.com/google/uuid"
)

// HandleImportUpload accepts a file upload, detects its format, generates a preview,
// and caches the parsed data for the execute step.
func (a *App) HandleImportUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Failed to parse upload: " + err.Error()})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "No file provided"})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Failed to read file"})
		return
	}

	filename := header.Filename
	ext := strings.ToLower(filepath.Ext(filename))

	// Parse the file into rows based on extension
	var rows [][]string
	switch ext {
	case ".xls":
		rows, err = datastore.ReadXLSFileFromBytes(fileBytes)
	case ".xlsx":
		rows, err = datastore.ParseUploadedXLSXRows(fileBytes)
	case ".csv":
		rows, err = datastore.ParseUploadedCSV(fileBytes)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Unsupported file type. Use .xls, .xlsx, or .csv"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("Failed to parse file: %v", err)})
		return
	}
	if len(rows) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "File has no data rows"})
		return
	}

	// Detect format
	format := datastore.DetectFormat(filename, rows[0])

	uploadID := uuid.New().String()
	entry := &UploadEntry{
		Format:    format,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if format == "namelist" {
		entries := datastore.ParseNamelist(rows)
		if len(entries) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"error": "No valid entries found in file"})
			return
		}

		preview, err := datastore.PreviewNamelistImport(a.DB, entries)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("Preview failed: %v", err)})
			return
		}
		preview.Filename = filename

		entry.Entries = entries
		a.storeUpload(uploadID, entry)

		writeJSON(w, http.StatusOK, map[string]any{
			"upload_id": uploadID,
			"format":    "namelist",
			"preview":   preview,
		})
		return
	}

	// Data format — parse into EntityData
	var entityData *datastore.EntityData
	if ext == ".xlsx" {
		entityData, err = datastore.ParseUploadedXLSXData(fileBytes)
	} else {
		// CSV: single entity type, detect from headers
		entityData = parseCSVAsEntityData(rows)
		err = nil
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("Parse failed: %v", err)})
		return
	}

	entry.Data = entityData
	a.storeUpload(uploadID, entry)

	writeJSON(w, http.StatusOK, map[string]any{
		"upload_id": uploadID,
		"format":    "data",
		"summary": map[string]int{
			"students":  len(entityData.Students),
			"parents":   len(entityData.Parents),
			"teachers":  len(entityData.Teachers),
			"rooms":     len(entityData.Rooms),
			"schedules": len(entityData.Schedules),
		},
		"filename": filename,
	})
}

// HandleImportExecute executes a previously previewed import.
func (a *App) HandleImportExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UploadID  string         `json:"upload_id"`
		Decisions map[int]string `json:"decisions"` // row_index -> "insert"|"merge"|"skip" (namelist only)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UploadID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "upload_id is required"})
		return
	}

	entry := a.getUpload(req.UploadID)
	if entry == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Upload expired or not found. Please upload the file again."})
		return
	}
	defer a.deleteUpload(req.UploadID)

	if entry.Format == "namelist" {
		result, err := datastore.ExecuteNamelistImport(a.DB, entry.Entries, req.Decisions)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": fmt.Sprintf("Import failed: %v", err)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"format":  "namelist",
			"message": fmt.Sprintf("Imported: %d inserted, %d merged, %d skipped, %d parents created, %d parents updated", result.StudentsInserted, result.StudentsMerged, result.StudentsSkipped, result.ParentsCreated, result.ParentsUpdated),
			"result":  result,
		})
		return
	}

	// Data format — bulk import
	if err := datastore.ImportAll(a.DB, entry.Data); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": fmt.Sprintf("Import failed: %v", err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"format": "data",
		"message": fmt.Sprintf("Imported %d students, %d parents, %d teachers, %d rooms, %d schedules",
			len(entry.Data.Students), len(entry.Data.Parents), len(entry.Data.Teachers), len(entry.Data.Rooms), len(entry.Data.Schedules)),
	})
}

// parseCSVAsEntityData detects entity type from CSV headers and parses accordingly.
func parseCSVAsEntityData(rows [][]string) *datastore.EntityData {
	if len(rows) < 1 {
		return &datastore.EntityData{}
	}

	headers := make(map[string]bool)
	for _, h := range rows[0] {
		headers[strings.ToLower(strings.TrimSpace(h))] = true
	}

	data := &datastore.EntityData{}

	// Detect entity type by characteristic headers
	switch {
	case headers["day_of_week"] || headers["start_time"]:
		data.Schedules = datastore.ExportParseScheduleRows(rows)
	case headers["capacity"]:
		data.Rooms = datastore.ExportParseRoomRows(rows)
	case headers["subjects"]:
		data.Teachers = datastore.ExportParseTeacherRows(rows)
	case headers["grade"] || headers["school"] || headers["parent_id"]:
		data.Students = datastore.ExportParseStudentRows(rows)
	default:
		// Default to parents
		data.Parents = datastore.ExportParseParentRows(rows)
	}

	return data
}
