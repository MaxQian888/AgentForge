package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DocxParser extracts text from DOCX files.
// DOCX files are ZIP archives containing word/document.xml with Office Open XML markup.
type DocxParser struct{}

// docxBody represents the <w:body> element in word/document.xml.
type docxBody struct {
	Paragraphs []docxParagraph `xml:"body>p"`
}

// docxParagraph represents a <w:p> element.
type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

// docxRun represents a <w:r> element containing text runs.
type docxRun struct {
	Text []docxText `xml:"t"`
}

// docxText represents a <w:t> element.
type docxText struct {
	Value string `xml:",chardata"`
}

const docxChunkSize = 2000

func (p *DocxParser) Parse(reader io.Reader) ([]TextChunk, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("docx: read input: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("docx: open zip: %w", err)
	}

	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return nil, fmt.Errorf("docx: word/document.xml not found in archive")
	}

	rc, err := docFile.Open()
	if err != nil {
		return nil, fmt.Errorf("docx: open document.xml: %w", err)
	}
	defer rc.Close()

	var body docxBody
	if err := xml.NewDecoder(rc).Decode(&body); err != nil {
		return nil, fmt.Errorf("docx: parse document.xml: %w", err)
	}

	// Extract text from paragraphs.
	paragraphs := make([]string, 0, len(body.Paragraphs))
	for _, para := range body.Paragraphs {
		var sb strings.Builder
		for _, run := range para.Runs {
			for _, t := range run.Text {
				sb.WriteString(t.Value)
			}
		}
		text := strings.TrimSpace(sb.String())
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}

	if len(paragraphs) == 0 {
		return []TextChunk{{
			Content:  "",
			Index:    0,
			Section:  "document",
			Metadata: map[string]string{"note": "no text content found"},
		}}, nil
	}

	// Merge small paragraphs into chunks of ~docxChunkSize characters.
	var chunks []TextChunk
	var current strings.Builder
	chunkIndex := 0

	for _, para := range paragraphs {
		if current.Len() > 0 && current.Len()+len(para)+1 > docxChunkSize {
			chunks = append(chunks, TextChunk{
				Content: current.String(),
				Index:   chunkIndex,
				Section: "document",
			})
			chunkIndex++
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(para)
	}
	if current.Len() > 0 {
		chunks = append(chunks, TextChunk{
			Content: current.String(),
			Index:   chunkIndex,
			Section: "document",
		})
	}

	return chunks, nil
}
