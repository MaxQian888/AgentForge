package document

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// TextChunk represents a parsed segment of text from a document.
type TextChunk struct {
	Content  string
	Index    int
	Section  string            // page name, sheet name, slide title
	Metadata map[string]string // optional extra metadata
}

// Parser defines the interface for document text extraction.
type Parser interface {
	Parse(reader io.Reader) ([]TextChunk, error)
}

// supportedExtensions maps file extensions to parser constructors.
var supportedExtensions = map[string]func() Parser{
	".docx": func() Parser { return &DocxParser{} },
	".xlsx": func() Parser { return &XlsxParser{} },
	".pptx": func() Parser { return &PptxParser{} },
	".pdf":  func() Parser { return &PDFParser{} },
}

// ParserForType returns a Parser for the given file extension (e.g. ".docx").
// Returns an error if the file type is not supported.
func ParserForType(fileType string) (Parser, error) {
	ext := strings.ToLower(fileType)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	constructor, ok := supportedExtensions[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported document type: %s", fileType)
	}
	return constructor(), nil
}

// ParserForFile returns a Parser based on the file name's extension.
func ParserForFile(fileName string) (Parser, error) {
	return ParserForType(filepath.Ext(fileName))
}

// SupportedTypes returns the list of supported file extensions.
func SupportedTypes() []string {
	types := make([]string, 0, len(supportedExtensions))
	for ext := range supportedExtensions {
		types = append(types, ext)
	}
	return types
}

// IsSupportedType checks whether the given extension is supported.
func IsSupportedType(fileType string) bool {
	ext := strings.ToLower(fileType)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	_, ok := supportedExtensions[ext]
	return ok
}
