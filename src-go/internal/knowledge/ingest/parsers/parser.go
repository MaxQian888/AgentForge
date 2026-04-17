// Package parsers re-exports the document parser interface and factory from
// the internal/document package so the ingest worker can use it without
// a circular import. New parser implementations should be added here.
package parsers

import (
	"io"

	"github.com/react-go-quick-starter/server/internal/document"
)

// TextChunk re-exports document.TextChunk for consumers of this package.
type TextChunk = document.TextChunk

// Parser is the document-text extraction interface.
type Parser = document.Parser

// ParserForFile returns a Parser based on the file name's extension.
func ParserForFile(fileName string) (Parser, error) {
	return document.ParserForFile(fileName)
}

// ParserForType returns a Parser for the given file extension (e.g. ".docx").
func ParserForType(fileType string) (Parser, error) {
	return document.ParserForType(fileType)
}

// SupportedTypes returns the list of supported file extensions.
func SupportedTypes() []string {
	return document.SupportedTypes()
}

// IsSupportedType checks whether the given extension is supported.
func IsSupportedType(fileType string) bool {
	return document.IsSupportedType(fileType)
}

// DocumentIngestParser adapts the document.Parser to the ingest worker's
// parsedChunk slice. Callers can use this to bridge the two interfaces.
type DocumentIngestParser struct {
	inner Parser
}

// NewDocumentIngestParser wraps a document.Parser.
func NewDocumentIngestParser(p Parser) *DocumentIngestParser {
	return &DocumentIngestParser{inner: p}
}

// Parse implements the ingest.ingestParser contract returning (index, content) pairs.
func (d *DocumentIngestParser) Parse(r io.Reader) ([]struct {
	Index   int
	Content string
}, error) {
	chunks, err := d.inner.Parse(r)
	if err != nil {
		return nil, err
	}
	out := make([]struct {
		Index   int
		Content string
	}, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, struct {
			Index   int
			Content string
		}{Index: c.Index, Content: c.Content})
	}
	return out, nil
}
