package ingest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"
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
		text := normalizeExtractedText(string(data))
		if text == "" {
			return "", errors.New("document is empty")
		}
		return text, nil
	case strings.HasSuffix(lowerName, ".pdf") || strings.Contains(strings.ToLower(mimeType), "pdf"):
		text := normalizeExtractedText(extractTextFromPDF(data))
		if !IsReadableText(text) {
			if fallback, err := extractTextFromPDFWithPyPDF(data); err == nil {
				cleanFallback := normalizeExtractedText(fallback)
				if IsReadableText(cleanFallback) || len(cleanFallback) > len(text) {
					text = cleanFallback
				}
			}
		}
		if text == "" {
			return "", errors.New("unable to extract text from pdf")
		}
		if !IsReadableText(text) {
			return "", errors.New("unable to extract readable text from pdf")
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

func extractTextFromPDFWithPyPDF(data []byte) (string, error) {
	const pyScript = `
import io
import sys

try:
    from pypdf import PdfReader
except Exception as e:
    raise RuntimeError(f"import pypdf failed: {e}")

raw = sys.stdin.buffer.read()
if not raw:
    print("")
    raise SystemExit(0)

reader = PdfReader(io.BytesIO(raw))
parts = []
for page in reader.pages:
    text = page.extract_text() or ""
    text = text.strip()
    if text:
        parts.append(text)

sys.stdout.write("\n".join(parts))
`
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", pyScript)
	cmd.Stdin = bytes.NewReader(data)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pypdf extraction failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
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

func normalizeExtractedText(in string) string {
	if strings.TrimSpace(in) == "" {
		return ""
	}
	in = strings.ReplaceAll(in, "\u00a0", " ")
	in = strings.ReplaceAll(in, "\u200b", "")
	in = norm.NFKC.String(in)
	return normalizeWhitespace(in)
}

// IsReadableText reports whether extracted text has enough readable characters
// to be used for retrieval and QA.
func IsReadableText(in string) bool {
	in = strings.TrimSpace(in)
	if in == "" {
		return false
	}

	runes := []rune(in)
	if len(runes) < 16 {
		return false
	}

	readable := 0
	for _, r := range runes {
		switch {
		case unicode.IsSpace(r):
			readable++
		case unicode.Is(unicode.Han, r):
			readable++
		case r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			readable++
		case strings.ContainsRune("，。！？；：,.!?;:()[]{}<>-_/#'\"%+*", r):
			readable++
		}
	}
	ratio := float64(readable) / float64(len(runes))
	return ratio >= 0.45
}
