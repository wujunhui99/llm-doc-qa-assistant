package ingest

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode/utf16"
)

var (
	pdfTextPattern       = regexp.MustCompile(`\(([^\)]*)\)\s*Tj`)
	pdfArrayPattern      = regexp.MustCompile(`\[(.*?)\]\s*TJ`)
	pdfArrayTokenPattern = regexp.MustCompile(`\(([^\)]*)\)`)
)

func ParseDocumentText(fileName, mimeType string, data []byte) (string, error) {
	lowerName := strings.ToLower(fileName)
	switch {
	case strings.HasSuffix(lowerName, ".txt"), strings.HasSuffix(lowerName, ".md"), strings.HasSuffix(lowerName, ".markdown"):
		text := strings.TrimSpace(string(data))
		if text == "" {
			return "", errors.New("document is empty")
		}
		return text, nil
	case strings.HasSuffix(lowerName, ".pdf") || strings.Contains(strings.ToLower(mimeType), "pdf"):
		text := strings.TrimSpace(extractTextFromPDF(data))
		if text == "" {
			return "", errors.New("unable to extract text from pdf")
		}
		return text, nil
	default:
		return "", errors.New("unsupported document type")
	}
}

// extractTextFromPDF provides lightweight text extraction for simple PDFs.
func extractTextFromPDF(data []byte) string {
	var parts []string
	body := string(data)

	for _, m := range pdfTextPattern.FindAllStringSubmatch(body, -1) {
		parts = append(parts, decodePDFLiteral(m[1]))
	}

	for _, arrMatch := range pdfArrayPattern.FindAllStringSubmatch(body, -1) {
		arrayBody := arrMatch[1]
		for _, token := range pdfArrayTokenPattern.FindAllStringSubmatch(arrayBody, -1) {
			parts = append(parts, decodePDFLiteral(token[1]))
		}
	}

	text := normalizeWhitespace(strings.Join(parts, " "))
	if text != "" {
		return text
	}

	// Fallback: if PDF includes readable UTF-16 text segments.
	utf16Text := extractUTF16Text(data)
	return normalizeWhitespace(utf16Text)
}

func decodePDFLiteral(in string) string {
	in = strings.ReplaceAll(in, `\\n`, "\n")
	in = strings.ReplaceAll(in, `\\r`, "\n")
	in = strings.ReplaceAll(in, `\\t`, "\t")
	in = strings.ReplaceAll(in, `\\(`, "(")
	in = strings.ReplaceAll(in, `\\)`, ")")
	in = strings.ReplaceAll(in, `\\\\`, `\\`)
	return in
}

func extractUTF16Text(data []byte) string {
	marker := []byte{0xFE, 0xFF}
	idx := bytes.Index(data, marker)
	if idx < 0 || idx+2 >= len(data) {
		return ""
	}

	var words []uint16
	for i := idx + 2; i+1 < len(data); i += 2 {
		w := uint16(data[i])<<8 | uint16(data[i+1])
		if w == 0 {
			continue
		}
		if w < 32 && w != '\n' && w != '\t' {
			continue
		}
		words = append(words, w)
		if len(words) > 12000 {
			break
		}
	}
	if len(words) == 0 {
		return ""
	}
	return string(utf16.Decode(words))
}

func normalizeWhitespace(in string) string {
	in = strings.ReplaceAll(in, "\r", "\n")
	lines := strings.Split(in, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	merged := strings.Join(lines, " ")
	return strings.Join(strings.Fields(merged), " ")
}
