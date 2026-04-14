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

// PptxParser extracts text from PPTX files.
// PPTX files are ZIP archives containing ppt/slides/slide*.xml.
type PptxParser struct{}

// pptxSlide represents the parsed content of a slide XML file.
// Text is stored in <a:t> elements nested within shape trees.
type pptxSlide struct {
	XMLName xml.Name    `xml:"sld"`
	CSld    pptxCSld    `xml:"cSld"`
}

type pptxCSld struct {
	SpTree pptxSpTree `xml:"spTree"`
}

type pptxSpTree struct {
	Shapes []pptxShape `xml:"sp"`
}

type pptxShape struct {
	TxBody *pptxTxBody `xml:"txBody"`
}

type pptxTxBody struct {
	Paragraphs []pptxParagraph `xml:"p"`
}

type pptxParagraph struct {
	Runs []pptxRun `xml:"r"`
}

type pptxRun struct {
	Text string `xml:"t"`
}

func (p *PptxParser) Parse(reader io.Reader) ([]TextChunk, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("pptx: read input: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("pptx: open zip: %w", err)
	}

	// Collect and sort slide files.
	type slideEntry struct {
		index int
		file  *zip.File
	}
	var slides []slideEntry
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			numStr := f.Name[len("ppt/slides/slide") : len(f.Name)-len(".xml")]
			idx, _ := strconv.Atoi(numStr)
			slides = append(slides, slideEntry{index: idx, file: f})
		}
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].index < slides[j].index })

	var chunks []TextChunk
	chunkIndex := 0
	for _, slide := range slides {
		text, err := parsePptxSlide(slide.file)
		if err != nil {
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		chunks = append(chunks, TextChunk{
			Content: text,
			Index:   chunkIndex,
			Section: fmt.Sprintf("Slide %d", slide.index),
		})
		chunkIndex++
	}

	if len(chunks) == 0 {
		return []TextChunk{{
			Content:  "",
			Index:    0,
			Section:  "presentation",
			Metadata: map[string]string{"note": "no text content found"},
		}}, nil
	}

	return chunks, nil
}

func parsePptxSlide(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	// Use a token-based approach to extract all <a:t> elements,
	// which is more robust than trying to match the exact nesting structure.
	var texts []string
	decoder := xml.NewDecoder(bytes.NewReader(content))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		if se, ok := token.(xml.StartElement); ok {
			if se.Name.Local == "t" && se.Name.Space == "http://schemas.openxmlformats.org/drawingml/2006/main" {
				var text string
				if err := decoder.DecodeElement(&text, &se); err == nil && strings.TrimSpace(text) != "" {
					texts = append(texts, strings.TrimSpace(text))
				}
			}
		}
	}

	return strings.Join(texts, "\n"), nil
}
