package datastore

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// DetectFormat determines whether a file is in "namelist" format (positional columns)
// or "data" format (header-based with "id" column). Returns "namelist" or "data".
func DetectFormat(filename string, firstRow []string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".xls" {
		return "namelist"
	}
	// Check if the first row has an "id" header → data format
	for _, h := range firstRow {
		if strings.EqualFold(strings.TrimSpace(h), "id") {
			return "data"
		}
	}
	return "namelist"
}

// ParseUploadedCSV parses CSV data from bytes into rows.
func ParseUploadedCSV(data []byte) ([][]string, error) {
	return csv.NewReader(bytes.NewReader(data)).ReadAll()
}

// ParseUploadedXLSXRows reads the first sheet of an XLSX file as raw rows.
func ParseUploadedXLSXRows(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil
	}
	return f.GetRows(sheets[0])
}

// ParseUploadedXLSXData parses an XLSX file with named sheets (Students, Parents, etc.)
// into EntityData, reusing the same parsing logic as ReadAll.
func ParseUploadedXLSXData(data []byte) (*EntityData, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ed := &EntityData{}

	if rows, err := getSheetRows(f, "Parents"); err == nil {
		ed.Parents = parseParentRows(rows)
	}
	if rows, err := getSheetRows(f, "Students"); err == nil {
		ed.Students = parseStudentRows(rows)
	}
	if rows, err := getSheetRows(f, "Teachers"); err == nil {
		ed.Teachers = parseTeacherRows(rows)
	}
	if rows, err := getSheetRows(f, "Rooms"); err == nil {
		ed.Rooms = parseRoomRows(rows)
	}
	if rows, err := getSheetRows(f, "Schedules"); err == nil {
		ed.Schedules = parseScheduleRows(rows)
	}

	return ed, nil
}
