package datastore

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/extrame/xls"
)

// ReadXLSFile reads a legacy .xls (OLE2/BIFF) file and returns rows as [][]string,
// matching the format used by the existing CSV/XLSX readers.
func ReadXLSFile(path string) ([][]string, error) {
	wb, err := xls.Open(path, "utf-8")
	if err != nil {
		return nil, err
	}

	sheet := wb.GetSheet(0)
	if sheet == nil {
		return nil, nil
	}

	var rows [][]string
	for i := 0; i <= int(sheet.MaxRow); i++ {
		cells := readXLSRow(sheet, i)
		rows = append(rows, cells)
	}
	return rows, nil
}

// readXLSRow safely reads a single row from a sheet, recovering from panics
// caused by the xls library accessing sparse internal data.
func readXLSRow(sheet *xls.WorkSheet, idx int) (cells []string) {
	defer func() {
		if r := recover(); r != nil {
			cells = nil
		}
	}()
	row := sheet.Row(idx)
	if row == nil {
		return nil
	}
	lastCol := row.LastCol()
	for c := 0; c < lastCol; c++ {
		cells = append(cells, strings.TrimSpace(row.Col(c)))
	}
	return cells
}

// ListXLSFiles returns .xls filenames in the given directory, sorted newest first.
func ListXLSFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type fileInfo struct {
		name    string
		modTime int64
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".xls") && !strings.HasSuffix(strings.ToLower(e.Name()), ".xlsx") {
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{name: e.Name(), modTime: info.ModTime().Unix()})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})

	var names []string
	for _, f := range files {
		names = append(names, f.name)
	}
	return names, nil
}

// XLSFilePath returns the full path to an .xls file in the given directory.
// It validates the filename to prevent path traversal.
func XLSFilePath(dir, filename string) string {
	clean := filepath.Base(filename)
	return filepath.Join(dir, clean)
}
