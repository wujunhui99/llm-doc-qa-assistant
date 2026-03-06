package ingest

import "strings"

func ChunkText(text string, chunkSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = 600
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = 120
	}

	runes := []rune(text)
	if len(runes) <= chunkSize {
		return []string{text}
	}

	step := chunkSize - overlap
	if step <= 0 {
		step = chunkSize
	}

	chunks := make([]string, 0, (len(runes)/step)+1)
	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}
