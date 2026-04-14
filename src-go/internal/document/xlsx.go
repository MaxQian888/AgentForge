package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// XlsxParser extracts text from XLSX files.
// XLSX files are ZIP archives containing xl/sharedStrings.xml and xl/worksheets/sheet*.xml.
type XlsxParser struct{}

// xlsxSST represents the shared string table.
type xlsxSST struct {
	Strings []xlsxSI `xml:"si"`
}

// xlsxSI represents a shared string item.
type xlsxSI struct {
	T string   `xml:"t"`
	R []xlsxR  `xml:"r"`
}

// xlsxR represents a rich text run inside a shared string.
type xlsxR struct {
	T string `xml:"t"`
}

// xlsxWorkbook represents the workbook.xml for extracting sheet names.
type xlsxWorkbook struct {
	Sheets []xlsxSheet `xml:"sheets>sheet"`
}

// xlsxSheet represents a <sheet> element in workbook.xml.
type xlsxSheet struct {
	Name    string `xml:"name,attr"`
	SheetID string `xml:"sheetId,attr"`
}

// xlsxSheetData represents the worksheet data.
type xlsxSheetData struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

// xlsxRow represents a row in the worksheet.
type xlsxRow struct {
	R     int        `xml:"r,attr"`
	Cells []xlsxCell `xml:"c"`
}

// xlsxCell represents a cell in a row.
type xlsxCell struct {
	R string `xml:"r,attr"` // cell reference like "A1"
	T string `xml:"t,attr"` // type: "s" for shared string
	V string `xml:"v"`      // value or shared string index
}

func (p *XlsxParser) Parse(reader io.Reader) ([]TextChunk, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("xlsx: read input: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("xlsx: open zip: %w", err)
	}

	// Build file map for quick lookup.
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}

	// Parse shared strings.
	sharedStrings := parseSharedStrings(files)

	// Parse workbook for sheet names.
	sheetNames := parseWorkbookSheetNames(files)

	// Find and sort sheet files.
	type sheetEntry struct {
		index int
		name  string
		file  *zip.File
	}
	var sheets []sheetEntry
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			// Extract sheet number from filename.
			numStr := f.Name[len("xl/worksheets/sheet") : len(f.Name)-len(".xml")]
			idx, _ := strconv.Atoi(numStr)
			name := fmt.Sprintf("Sheet%d", idx)
			if idx > 0 && idx <= len(sheetNames) {
				name = sheetNames[idx-1]
			}
			sheets = append(sheets, sheetEntry{index: idx, name: name, file: f})
		}
	}
	sort.Slice(sheets, func(i, j int) bool { return sheets[i].index < sheets[j].index })

	// Parse each sheet.
	var chunks []TextChunk
	chunkIndex := 0
	for _, sheet := range sheets {
		content, err := parseXlsxSheet(sheet.file, sharedStrings)
		if err != nil {
			continue // skip unparseable sheets
		}
		if strings.TrimSpace(content) == "" {
			continue
		}
		chunks = append(chunks, TextChunk{
			Content: content,
			Index:   chunkIndex,
			Section: sheet.name,
		})
		chunkIndex++
	}

	if len(chunks) == 0 {
		return []TextChunk{{
			Content:  "",
			Index:    0,
			Section:  "workbook",
			Metadata: map[string]string{"note": "no text content found"},
		}}, nil
	}

	return chunks, nil
}

func parseSharedStrings(files map[string]*zip.File) []string {
	f, ok := files["xl/sharedStrings.xml"]
	if !ok {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	var sst xlsxSST
	if err := xml.NewDecoder(rc).Decode(&sst); err != nil {
		return nil
	}

	strings := make([]string, len(sst.Strings))
	for i, si := range sst.Strings {
		if si.T != "" {
			strings[i] = si.T
		} else {
			// Rich text: concatenate all runs.
			var sb bytes.Buffer
			for _, r := range si.R {
				sb.WriteString(r.T)
			}
			strings[i] = sb.String()
		}
	}
	return strings
}

func parseWorkbookSheetNames(files map[string]*zip.File) []string {
	f, ok := files["xl/workbook.xml"]
	if !ok {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	var wb xlsxWorkbook
	if err := xml.NewDecoder(rc).Decode(&wb); err != nil {
		return nil
	}

	names := make([]string, len(wb.Sheets))
	for i, s := range wb.Sheets {
		names[i] = s.Name
	}
	return names
}

func parseXlsxSheet(f *zip.File, sharedStrings []string) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var sheet xlsxSheetData
	if err := xml.NewDecoder(rc).Decode(&sheet); err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, row := range sheet.Rows {
		var cells []string
		for _, cell := range row.Cells {
			value := cell.V
			if cell.T == "s" {
				// Shared string reference.
				idx, err := strconv.Atoi(value)
				if err == nil && idx >= 0 && idx < len(sharedStrings) {
					value = sharedStrings[idx]
				}
			}
			if value != "" {
				ref := cell.R
				if ref == "" {
					ref = "?"
				}
				cells = append(cells, fmt.Sprintf("%s: %s", ref, value))
			}
		}
		if len(cells) > 0 {
			sb.WriteString(strings.Join(cells, ", "))
			sb.WriteString("\n")
		}
	}

	return strings.TrimSpace(sb.String()), nil
}
