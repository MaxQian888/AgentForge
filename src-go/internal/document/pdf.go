package document

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	gopdf "github.com/ledongthuc/pdf"
)

// PDFParser extracts text from PDF files using the ledongthuc/pdf library.
type PDFParser struct{}

func (p *PDFParser) Parse(r io.Reader) ([]TextChunk, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}
	reader, err := gopdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	var chunks []TextChunk
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(text)
		if content == "" {
			continue
		}
		chunks = append(chunks, TextChunk{
			Content:  content,
			Index:    i - 1,
			Section:  fmt.Sprintf("Page %d", i),
			Metadata: map[string]string{"page": fmt.Sprintf("%d", i)},
		})
	}
	if len(chunks) == 0 {
		return []TextChunk{{
			Content:  "PDF document contains no extractable text content.",
			Index:    0,
			Section:  "document",
			Metadata: map[string]string{"note": "no-text-extracted"},
		}}, nil
	}
	return chunks, nil
}
