package qa

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"llm-doc-qa-assistant/backend/internal/types"
)

var splitPattern = regexp.MustCompile(`[^\p{L}\p{N}]+`)

type ScoredChunk struct {
	Chunk    types.Chunk
	Document types.Document
	Score    int
}

func RetrieveTopChunks(question string, docs []types.Document, chunksByDoc map[string][]types.Chunk, topK int) []ScoredChunk {
	if topK <= 0 {
		topK = 4
	}
	qTokens := tokenize(question)
	if len(qTokens) == 0 {
		qTokens = []string{question}
	}

	results := make([]ScoredChunk, 0, topK*2)
	for _, doc := range docs {
		chunks := chunksByDoc[doc.ID]
		for _, chunk := range chunks {
			score := scoreChunk(qTokens, tokenize(chunk.Content), chunk.Content)
			if score <= 0 {
				continue
			}
			results = append(results, ScoredChunk{Chunk: chunk, Document: doc, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			if results[i].Document.ID == results[j].Document.ID {
				return results[i].Chunk.Index < results[j].Chunk.Index
			}
			return results[i].Document.ID < results[j].Document.ID
		}
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func scoreChunk(questionTokens, chunkTokens []string, rawChunk string) int {
	if len(chunkTokens) == 0 {
		return 0
	}

	chunkSet := make(map[string]int, len(chunkTokens))
	for _, t := range chunkTokens {
		chunkSet[t]++
	}

	score := 0
	for _, q := range questionTokens {
		if c, ok := chunkSet[q]; ok {
			score += 10 + min(c, 4)
		}
	}

	// Give small score for direct substring matches in Chinese or exact phrases.
	if strings.Contains(strings.ToLower(rawChunk), strings.ToLower(strings.Join(questionTokens, ""))) {
		score += 3
	}
	if len(rawChunk) < 120 {
		score -= 2
	}
	return score
}

func tokenize(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}
	parts := splitPattern.Split(text, -1)
	out := make([]string, 0, len(parts)*2)
	seen := make(map[string]struct{}, len(parts)*2)

	appendToken := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}

	for _, p := range parts {
		if p == "" {
			continue
		}
		if len([]rune(p)) >= 2 {
			appendToken(p)
		}
	}

	// Add CJK bigrams to improve Chinese matching.
	hanRunes := make([]rune, 0, len([]rune(text)))
	for _, r := range []rune(text) {
		if unicode.Is(unicode.Han, r) {
			hanRunes = append(hanRunes, r)
		}
	}
	if len(hanRunes) > 0 {
		appendToken(string(hanRunes))
		for i := 0; i < len(hanRunes)-1; i++ {
			appendToken(string(hanRunes[i : i+2]))
		}
	}

	if len(out) == 0 {
		appendToken(text)
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
