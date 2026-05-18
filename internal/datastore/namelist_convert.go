package datastore

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// canonicalNamelistHeaders is the 16-column positional header row that
// ParseNamelist (see namelist.go) expects. Index 0 is the sparse Group label
// column; index 1 is a row-sequence column ignored by the parser.
var canonicalNamelistHeaders = []string{
	"Group", "Index",
	"Student Name", "English Name",
	"School", "Grade", "Package",
	"Student Email", "Student Phone",
	"Parent Name", "Parent Email", "Parent Phone",
	"Home Address",
	"Major", "Enroll", "Graduate",
}

// namelistHeaderAliases lists the accepted source header strings for each
// canonical output column, in priority order. Keys are output-column indexes.
// Matching is case-insensitive and whitespace-collapsed (see headerKey).
var namelistHeaderAliases = map[int][]string{
	0:  {"group"},
	2:  {"student name"},
	3:  {"english name"},
	4:  {"school"},
	5:  {"grade"},
	6:  {"package"},
	7:  {"student email", "email"},
	8:  {"student phone", "phone"},
	9:  {"parent name"},
	10: {"parent email"},
	11: {"parent phone"},
	12: {"home address", "address"},
	13: {"major"},
	14: {"enroll", "enroll term"},
	15: {"graduate", "graduation"},
}

// headerKey normalizes a header cell for matching: lowercased, with internal
// runs of whitespace collapsed to a single space and edges trimmed.
func headerKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ConvertHeaderNamelistRows reprojects rows from a header-bearing namelist
// (e.g. the 2026 student namelist) into the 16-column positional shape that
// ParseNamelist consumes. Columns are matched by header text, so reordered or
// absent source columns are tolerated; missing columns yield empty strings.
//
// Rows whose Student Name and English Name are both empty are dropped, matching
// ParseNamelist's own skip rule.
func ConvertHeaderNamelistRows(rows [][]string) ([][]string, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("namelist has no data rows")
	}

	headerRow := rows[0]
	srcIdx := make(map[string]int, len(headerRow))
	for i, h := range headerRow {
		k := headerKey(h)
		if k == "" {
			continue
		}
		if _, dup := srcIdx[k]; !dup {
			srcIdx[k] = i
		}
	}

	colSrc := make([]int, len(canonicalNamelistHeaders))
	for i := range colSrc {
		colSrc[i] = -1
	}
	for out, aliases := range namelistHeaderAliases {
		for _, a := range aliases {
			if i, ok := srcIdx[a]; ok {
				colSrc[out] = i
				break
			}
		}
	}

	if colSrc[2] == -1 && colSrc[3] == -1 {
		return nil, fmt.Errorf("namelist needs a 'Student Name' or 'English Name' column")
	}

	out := make([][]string, 0, len(rows))
	out = append(out, append([]string(nil), canonicalNamelistHeaders...))

	seq := 0
	for _, r := range rows[1:] {
		row := make([]string, len(canonicalNamelistHeaders))
		for k, s := range colSrc {
			if s == -1 || s >= len(r) {
				continue
			}
			row[k] = strings.TrimSpace(r[s])
		}
		if row[2] == "" && row[3] == "" {
			continue
		}
		seq++
		row[1] = strconv.Itoa(seq)
		out = append(out, row)
	}

	return out, nil
}

// ConvertNamelistXLSX reads a header-bearing namelist XLSX at srcPath, converts
// it to the positional layout, and writes the result to dstPath as an XLSX file
// suitable for the existing /admin/import upload flow. Returns the number of
// data rows written (excluding the header).
func ConvertNamelistXLSX(srcPath, dstPath string) (int, error) {
	src, err := excelize.OpenFile(srcPath)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer src.Close()

	sheets := src.GetSheetList()
	if len(sheets) == 0 {
		return 0, fmt.Errorf("%s has no sheets", srcPath)
	}
	rows, err := src.GetRows(sheets[0])
	if err != nil {
		return 0, fmt.Errorf("read sheet %s: %w", sheets[0], err)
	}

	converted, err := ConvertHeaderNamelistRows(rows)
	if err != nil {
		return 0, err
	}

	dst := excelize.NewFile()
	defer dst.Close()
	sheet := dst.GetSheetName(0)
	for r, row := range converted {
		cell, err := excelize.CoordinatesToCellName(1, r+1)
		if err != nil {
			return 0, err
		}
		if err := dst.SetSheetRow(sheet, cell, &row); err != nil {
			return 0, err
		}
	}
	if err := dst.SaveAs(dstPath); err != nil {
		return 0, fmt.Errorf("save %s: %w", dstPath, err)
	}

	return len(converted) - 1, nil
}
