package datastore

import (
	"os"
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
	for i := range int(sheet.MaxRow) + 1 {
		cells := readXLSRow(sheet, i)
		rows = append(rows, cells)
	}
	return rows, nil
}

// ReadXLSFileFromBytes writes bytes to a temp file, parses as .xls, then cleans up.
// The extrame/xls library requires a file path.
func ReadXLSFileFromBytes(data []byte) ([][]string, error) {
	tmp, err := os.CreateTemp("", "classgo-import-*.xls")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	return ReadXLSFile(tmpPath)
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
	for c := range lastCol {
		cells = append(cells, strings.TrimSpace(row.Col(c)))
	}
	return cells
}
