package document

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// PDFParser provides basic text extraction from PDF files.
// PDF is a complex format; this implementation uses a simplified approach
// that scans for text streams between BT/ET operators and extracts Tj/TJ operands.
// For production use, consider replacing with a dedicated PDF library.
type PDFParser struct{}

func (p *PDFParser) Parse(reader io.Reader) ([]TextChunk, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("pdf: read input: %w", err)
	}

	// Verify PDF magic bytes.
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return nil, fmt.Errorf("pdf: not a valid PDF file")
	}

	text := extractPDFText(data)
	if strings.TrimSpace(text) == "" {
		return []TextChunk{{
			Content:  "PDF text extraction requires manual review",
			Index:    0,
			Section:  "document",
			Metadata: map[string]string{"note": "basic extraction yielded no text; document may contain scanned images or complex encoding"},
		}}, nil
	}

	return []TextChunk{{
		Content: text,
		Index:   0,
		Section: "document",
	}}, nil
}

// extractPDFText performs a simplified scan of PDF content streams.
// It looks for text between BT (Begin Text) and ET (End Text) operators,
// then extracts string operands from Tj and TJ operators.
func extractPDFText(data []byte) string {
	var result strings.Builder
	content := string(data)

	// Find all BT...ET blocks.
	for {
		btIdx := strings.Index(content, "BT")
		if btIdx < 0 {
			break
		}
		content = content[btIdx+2:]
		etIdx := strings.Index(content, "ET")
		if etIdx < 0 {
			break
		}
		block := content[:etIdx]
		content = content[etIdx+2:]

		// Extract text from Tj and TJ operators within this block.
		extracted := extractTextFromBlock(block)
		if extracted != "" {
			if result.Len() > 0 {
				result.WriteString(" ")
			}
			result.WriteString(extracted)
		}
	}

	return strings.TrimSpace(result.String())
}

// extractTextFromBlock extracts readable strings from a PDF text block.
// Looks for (string) Tj operators and [(string)] TJ array operators.
func extractTextFromBlock(block string) string {
	var texts []string

	for i := 0; i < len(block); i++ {
		if block[i] == '(' {
			// Find matching closing parenthesis, handling escaped parens.
			depth := 1
			start := i + 1
			j := start
			for j < len(block) && depth > 0 {
				if block[j] == '\\' {
					j++ // skip escaped character
				} else if block[j] == '(' {
					depth++
				} else if block[j] == ')' {
					depth--
				}
				if depth > 0 {
					j++
				}
			}
			if depth == 0 {
				text := unescapePDFString(block[start:j])
				if isPrintableText(text) {
					texts = append(texts, text)
				}
				i = j
			}
		}
	}

	return strings.Join(texts, " ")
}

// unescapePDFString handles basic PDF string escape sequences.
func unescapePDFString(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			case '(':
				result.WriteByte('(')
			case ')':
				result.WriteByte(')')
			case '\\':
				result.WriteByte('\\')
			default:
				result.WriteByte(s[i])
			}
		} else {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// isPrintableText checks if a string contains mostly printable characters.
func isPrintableText(s string) bool {
	if len(s) == 0 {
		return false
	}
	printable := 0
	for _, r := range s {
		if r >= 32 && r < 127 || r == '\n' || r == '\r' || r == '\t' {
			printable++
		}
	}
	return float64(printable)/float64(len(s)) > 0.5
}
