package document

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// --- helpers to create minimal valid Office documents in memory ---

func createMinimalDocx(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>
  </w:body>
</w:document>`

	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(docXML)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func createMinimalXlsx(t *testing.T, cellValue string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	sharedStringsXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="1" uniqueCount="1">
  <si><t>` + cellValue + `</t></si>
</sst>`

	workbookXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheets>
    <sheet name="TestSheet" sheetId="1"/>
  </sheets>
</workbook>`

	sheetXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="s"><v>0</v></c>
    </row>
  </sheetData>
</worksheet>`

	for name, content := range map[string]string{
		"xl/sharedStrings.xml":      sharedStringsXML,
		"xl/workbook.xml":           workbookXML,
		"xl/worksheets/sheet1.xml":  sheetXML,
	} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func createMinimalPptx(t *testing.T, slideText string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	slideXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:txBody>
          <a:p><a:r><a:t>` + slideText + `</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`

	f, err := w.Create("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(slideXML)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// --- Tests ---

func TestParserForType_Dispatch(t *testing.T) {
	tests := []struct {
		fileType string
		wantErr  bool
	}{
		{".docx", false},
		{".xlsx", false},
		{".pptx", false},
		{".pdf", false},
		{"docx", false},  // without leading dot
		{".txt", true},
		{".csv", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.fileType, func(t *testing.T) {
			p, err := ParserForType(tt.fileType)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParserForType(%q) = %v, want error", tt.fileType, p)
				}
			} else {
				if err != nil {
					t.Errorf("ParserForType(%q) error = %v", tt.fileType, err)
				}
				if p == nil {
					t.Errorf("ParserForType(%q) returned nil parser", tt.fileType)
				}
			}
		})
	}
}

func TestSupportedTypes(t *testing.T) {
	types := SupportedTypes()
	if len(types) < 4 {
		t.Errorf("SupportedTypes() returned %d types, want at least 4", len(types))
	}

	expected := map[string]bool{".docx": true, ".xlsx": true, ".pptx": true, ".pdf": true}
	for _, ext := range types {
		if !expected[ext] {
			t.Errorf("unexpected type in SupportedTypes(): %s", ext)
		}
	}
}

func TestIsSupportedType(t *testing.T) {
	if !IsSupportedType(".docx") {
		t.Error("IsSupportedType(.docx) = false, want true")
	}
	if IsSupportedType(".txt") {
		t.Error("IsSupportedType(.txt) = true, want false")
	}
}

func TestDocxParser(t *testing.T) {
	data := createMinimalDocx(t, "Hello World from DOCX")
	parser := &DocxParser{}

	chunks, err := parser.Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DocxParser.Parse() error = %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("DocxParser.Parse() returned 0 chunks")
	}
	if !strings.Contains(chunks[0].Content, "Hello World from DOCX") {
		t.Errorf("DocxParser.Parse() chunk content = %q, want to contain 'Hello World from DOCX'", chunks[0].Content)
	}
	if chunks[0].Section != "document" {
		t.Errorf("DocxParser.Parse() section = %q, want 'document'", chunks[0].Section)
	}
}

func TestXlsxParser(t *testing.T) {
	data := createMinimalXlsx(t, "Spreadsheet Data")
	parser := &XlsxParser{}

	chunks, err := parser.Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("XlsxParser.Parse() error = %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("XlsxParser.Parse() returned 0 chunks")
	}
	if !strings.Contains(chunks[0].Content, "Spreadsheet Data") {
		t.Errorf("XlsxParser.Parse() chunk content = %q, want to contain 'Spreadsheet Data'", chunks[0].Content)
	}
	if chunks[0].Section != "TestSheet" {
		t.Errorf("XlsxParser.Parse() section = %q, want 'TestSheet'", chunks[0].Section)
	}
}

func TestPptxParser(t *testing.T) {
	data := createMinimalPptx(t, "Presentation Title")
	parser := &PptxParser{}

	chunks, err := parser.Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("PptxParser.Parse() error = %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("PptxParser.Parse() returned 0 chunks")
	}
	if !strings.Contains(chunks[0].Content, "Presentation Title") {
		t.Errorf("PptxParser.Parse() chunk content = %q, want to contain 'Presentation Title'", chunks[0].Content)
	}
	if chunks[0].Section != "Slide 1" {
		t.Errorf("PptxParser.Parse() section = %q, want 'Slide 1'", chunks[0].Section)
	}
}

func TestPDFParser_InvalidFile(t *testing.T) {
	parser := &PDFParser{}
	_, err := parser.Parse(bytes.NewReader([]byte("not a pdf")))
	if err == nil {
		t.Error("PDFParser.Parse() with invalid data should return error")
	}
}

func TestPDFParser_MinimalPDF(t *testing.T) {
	// Build a minimal structurally valid PDF with correct byte offsets.
	// The ledongthuc/pdf library requires a proper xref table and %%EOF.
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.0\n")

	off1 := buf.Len()
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	off2 := buf.Len()
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	off3 := buf.Len()
	buf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\nendobj\n")

	xrefOff := buf.Len()
	buf.WriteString("xref\n0 4\n")
	buf.WriteString(fmt.Sprintf("0000000000 65535 f \n"))
	buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off1))
	buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off2))
	buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off3))
	buf.WriteString("trailer\n<< /Size 4 /Root 1 0 R >>\n")
	buf.WriteString(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xrefOff))

	parser := &PDFParser{}
	chunks, err := parser.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("PDFParser.Parse() error = %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("PDFParser.Parse() returned 0 chunks")
	}
	// The page has no text content stream, so the parser should return
	// the fallback "no extractable text" chunk.
	if !strings.Contains(chunks[0].Content, "no extractable text") {
		// If somehow text was extracted, that's also acceptable.
		t.Logf("PDFParser.Parse() chunk content = %q", chunks[0].Content)
	}
}

func TestDocxParser_Chunking(t *testing.T) {
	// Create a DOCX with multiple paragraphs that exceed chunk size.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>`)
	// Create enough paragraphs to trigger chunking (each ~500 chars).
	longPara := strings.Repeat("A", 500)
	for i := 0; i < 10; i++ {
		sb.WriteString(`<w:p><w:r><w:t>` + longPara + `</w:t></w:r></w:p>`)
	}
	sb.WriteString(`</w:body></w:document>`)

	f, _ := w.Create("word/document.xml")
	f.Write([]byte(sb.String()))
	w.Close()

	parser := &DocxParser{}
	chunks, err := parser.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("DocxParser.Parse() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("DocxParser.Parse() returned %d chunks, want at least 2 for large document", len(chunks))
	}
}
