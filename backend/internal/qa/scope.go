package qa

import (
	"errors"
	"regexp"
	"sort"
	"strings"

	"llm-doc-qa-assistant/backend/internal/types"
)

var docPrefixPattern = regexp.MustCompile(`^@doc\(([^\)]*)\)\s*`)

type Scope struct {
	Type         string   `json:"type"`
	DocIDs       []string `json:"doc_ids"`
	QuestionBody string   `json:"question_body"`
}

func ResolveScope(message, explicitType string, explicitDocIDs []string, docs []types.Document) (Scope, error) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return Scope{}, errors.New("message cannot be empty")
	}

	// Explicit API scope has highest priority.
	if explicitType != "" {
		s := Scope{Type: explicitType, DocIDs: uniqueStrings(explicitDocIDs), QuestionBody: trimmed}
		if s.Type == "doc" && len(s.DocIDs) == 0 {
			return Scope{}, errors.New("doc scope requires at least one document id")
		}
		if s.Type != "all" && s.Type != "doc" {
			return Scope{}, errors.New("scope_type must be all or doc")
		}
		return s, nil
	}

	if strings.HasPrefix(strings.ToLower(trimmed), "@all") {
		question := strings.TrimSpace(trimmed[4:])
		if question == "" {
			question = "请基于我的文档回答。"
		}
		return Scope{Type: "all", QuestionBody: question}, nil
	}

	if m := docPrefixPattern.FindStringSubmatch(trimmed); len(m) == 2 {
		raw := strings.TrimSpace(m[1])
		if raw == "" {
			return Scope{}, errors.New("@doc() must include one or more document references")
		}
		refs := parseCommaList(raw)
		idSet := make(map[string]struct{})
		for _, ref := range refs {
			for _, doc := range docs {
				if strings.EqualFold(doc.ID, ref) || strings.EqualFold(doc.Name, ref) {
					idSet[doc.ID] = struct{}{}
				}
			}
		}
		docIDs := mapKeys(idSet)
		if len(docIDs) == 0 {
			return Scope{}, errors.New("@doc references do not match owned documents")
		}
		question := strings.TrimSpace(strings.TrimPrefix(trimmed, m[0]))
		if question == "" {
			question = "请基于选中文档回答。"
		}
		return Scope{Type: "doc", DocIDs: docIDs, QuestionBody: question}, nil
	}

	return Scope{Type: "all", QuestionBody: trimmed}, nil
}

func parseCommaList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	set := make(map[string]struct{}, len(in))
	for _, item := range in {
		if item == "" {
			continue
		}
		set[item] = struct{}{}
	}
	return mapKeys(set)
}

func mapKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
