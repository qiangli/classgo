package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"classgo/internal/datastore"
)

// HandleNamelistFiles returns a list of .xls files in the raw/ directory.
func (a *App) HandleNamelistFiles(w http.ResponseWriter, r *http.Request) {
	rawDir := a.RawDir
	files, err := datastore.ListXLSFiles(rawDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("Failed to list files: %v", err)})
		return
	}
	if files == nil {
		files = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

// HandleNamelistPreview parses a namelist .xls file and returns a preview
// with conflict detection against existing students.
func (a *App) HandleNamelistPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Filename == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "filename is required"})
		return
	}

	rawDir := a.RawDir
	path := datastore.XLSFilePath(rawDir, req.Filename)

	rows, err := datastore.ReadXLSFile(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("Failed to read file: %v", err)})
		return
	}

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
	preview.Filename = req.Filename

	writeJSON(w, http.StatusOK, preview)
}

// HandleNamelistExecute executes a namelist import with the given decisions.
func (a *App) HandleNamelistExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Filename  string         `json:"filename"`
		Decisions map[int]string `json:"decisions"` // row_index -> "insert"|"merge"|"skip"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Filename == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "filename and decisions are required"})
		return
	}

	rawDir := a.RawDir
	path := datastore.XLSFilePath(rawDir, req.Filename)

	rows, err := datastore.ReadXLSFile(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("Failed to read file: %v", err)})
		return
	}

	entries := datastore.ParseNamelist(rows)
	if len(entries) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "No valid entries found"})
		return
	}

	result, err := datastore.ExecuteNamelistImport(a.DB, entries, req.Decisions)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": fmt.Sprintf("Import failed: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": fmt.Sprintf("Imported: %d inserted, %d merged, %d skipped, %d parents created, %d parents updated", result.StudentsInserted, result.StudentsMerged, result.StudentsSkipped, result.ParentsCreated, result.ParentsUpdated),
		"result":  result,
	})
}
