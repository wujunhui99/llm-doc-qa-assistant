package llm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type ContextChunk struct {
	DocID      string
	DocName    string
	ChunkID    string
	ChunkIndex int
	Score      int
	Content    string
}

type Request struct {
	OwnerUserID string
	ThreadID    string
	TurnID      string

	Question             string
	ScopeType            string
	ScopeDocIDs          []string
	Contexts             []ContextChunk
	PreviousTurnQuestion string
	PreviousTurnAnswer   string
	ActiveProvider       string
}

type Generator interface {
	GenerateAnswer(ctx context.Context, req Request) (string, error)
}

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type ChatCompleter interface {
	Available() bool
	ChatCompletion(ctx context.Context, messages []ChatMessage, model string, temperature float64) (string, error)
}

type Config struct {
	DefaultProvider   string
	ChatModel         string
	Temperature       float64
	MaxContextChunks  int
	RequestTimeout    time.Duration
	ProviderChatModel map[string]string
}

type Agent struct {
	chat     ChatCompleter
	embedder Embedder
	cfg      Config
}

func NewAgent(chat ChatCompleter, embedder Embedder, cfg Config) *Agent {
	if cfg.MaxContextChunks <= 0 {
		cfg.MaxContextChunks = 6
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 20 * time.Second
	}
	if strings.TrimSpace(cfg.ChatModel) == "" {
		cfg.ChatModel = "Pro/MiniMaxAI/MiniMax-M2.5"
	}
	if strings.TrimSpace(cfg.DefaultProvider) == "" {
		cfg.DefaultProvider = "siliconflow"
	}
	if cfg.ProviderChatModel == nil {
		cfg.ProviderChatModel = map[string]string{}
	}
	return &Agent{
		chat:     chat,
		embedder: embedder,
		cfg:      cfg,
	}
}

func (a *Agent) GenerateAnswer(ctx context.Context, req Request) (string, error) {
	question := strings.TrimSpace(req.Question)
	prevQ := strings.TrimSpace(req.PreviousTurnQuestion)
	prevA := strings.TrimSpace(req.PreviousTurnAnswer)

	if len(req.Contexts) == 0 {
		return a.generateWithoutContexts(ctx, req, question, prevQ, prevA)
	}

	selected := a.selectContexts(ctx, question, req.Contexts)
	if len(selected) == 0 {
		selected = req.Contexts[:minInt(a.cfg.MaxContextChunks, len(req.Contexts))]
	}

	if a.chat == nil || !a.chat.Available() {
		return "", ErrUnavailable
	}

	model := a.resolveModel(req.ActiveProvider)
	messages := buildMessages(question, prevQ, prevA, req.ScopeType, selected)

	callCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()

	answer, err := a.chat.ChatCompletion(callCtx, messages, model, a.cfg.Temperature)
	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", errors.New("chat completion returned empty answer")
	}
	return answer, nil
}

func (a *Agent) generateWithoutContexts(ctx context.Context, req Request, question, prevQ, prevA string) (string, error) {
	if a.chat == nil || !a.chat.Available() {
		return "", ErrUnavailable
	}

	model := a.resolveModel(req.ActiveProvider)
	messages := buildNoContextMessages(question, prevQ, prevA, req.ScopeType)
	callCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()

	answer, err := a.chat.ChatCompletion(callCtx, messages, model, a.cfg.Temperature)
	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", errors.New("chat completion returned empty answer")
	}
	return answer, nil
}

func (a *Agent) resolveModel(activeProvider string) string {
	key := strings.ToLower(strings.TrimSpace(activeProvider))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(a.cfg.DefaultProvider))
	}
	if m, ok := a.cfg.ProviderChatModel[key]; ok && strings.TrimSpace(m) != "" {
		return m
	}
	return a.cfg.ChatModel
}

func (a *Agent) selectContexts(ctx context.Context, question string, contexts []ContextChunk) []ContextChunk {
	limit := minInt(a.cfg.MaxContextChunks, len(contexts))
	if limit <= 0 {
		return nil
	}
	if strings.TrimSpace(question) == "" || a.embedder == nil {
		return append([]ContextChunk(nil), contexts[:limit]...)
	}

	inputs := make([]string, 0, len(contexts)+1)
	inputs = append(inputs, question)
	for _, c := range contexts {
		inputs = append(inputs, strings.TrimSpace(c.Content))
	}
	vectors, err := a.embedder.Embed(ctx, inputs)
	if err != nil || len(vectors) != len(inputs) {
		return append([]ContextChunk(nil), contexts[:limit]...)
	}

	qVec := vectors[0]
	type scored struct {
		chunk ContextChunk
		score float64
		idx   int
	}
	scoredList := make([]scored, 0, len(contexts))
	for i, c := range contexts {
		score := cosineSimilarity(qVec, vectors[i+1])
		scoredList = append(scoredList, scored{
			chunk: c,
			score: score,
			idx:   i,
		})
	}
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score == scoredList[j].score {
			return scoredList[i].idx < scoredList[j].idx
		}
		return scoredList[i].score > scoredList[j].score
	})

	out := make([]ContextChunk, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scoredList[i].chunk)
	}
	return out
}

func buildMessages(question, previousQuestion, previousAnswer, scopeType string, contexts []ContextChunk) []ChatMessage {
	scopeType = strings.TrimSpace(scopeType)
	if scopeType == "" {
		scopeType = "all"
	}

	contextLines := make([]string, 0, len(contexts))
	for i, c := range contexts {
		docName := strings.TrimSpace(c.DocName)
		if docName == "" {
			docName = c.DocID
		}
		excerpt := truncate(strings.ReplaceAll(strings.TrimSpace(c.Content), "\n", " "), 240)
		contextLines = append(contextLines, fmt.Sprintf("[%d] %s#%d: %s", i+1, docName, c.ChunkIndex, excerpt))
	}

	return []ChatMessage{
		{
			Role: "system",
			Content: "你是严谨的文档问答助手。只能根据给定证据回答，不允许编造。" +
				"回答中尽量引用证据编号[1][2]...，并保持中文表达清晰连贯。",
		},
		{
			Role: "user",
			Content: fmt.Sprintf(
				"作用域: %s\n上一轮问题: %s\n上一轮答案: %s\n当前问题: %s\n证据:\n%s",
				scopeType,
				previousQuestion,
				previousAnswer,
				question,
				strings.Join(contextLines, "\n"),
			),
		},
	}
}

func buildNoContextMessages(question, previousQuestion, previousAnswer, scopeType string) []ChatMessage {
	scopeType = strings.TrimSpace(scopeType)
	if scopeType == "" {
		scopeType = "all"
	}

	return []ChatMessage{
		{
			Role: "system",
			Content: "你是文档问答助手。当前没有检索到任何文档证据。" +
				"你可以进行简短通用对话，但要明确说明回答不基于文档证据。" +
				"如果用户询问文档内容，请提示其上传文档或切换到合适的 @all/@doc 作用域。",
		},
		{
			Role: "user",
			Content: fmt.Sprintf(
				"作用域: %s\n上一轮问题: %s\n上一轮答案: %s\n当前问题: %s\n当前证据: 无",
				scopeType,
				previousQuestion,
				previousAnswer,
				question,
			),
		},
	}
}

func fallbackNoContext(previousAnswer string) string {
	previousAnswer = strings.TrimSpace(previousAnswer)
	if previousAnswer != "" {
		return "未检索到新证据，先延续上一轮结论：" + truncate(previousAnswer, 140) + "。建议切换到 @all 或补充更具体关键词。"
	}
	return "当前作用域未检索到足够证据，请切换作用域或补充关键词后重试。"
}

func fallbackAnswer(question, previousAnswer string, contexts []ContextChunk) string {
	lines := make([]string, 0, 6)
	previousAnswer = strings.TrimSpace(previousAnswer)
	question = strings.TrimSpace(question)
	if previousAnswer != "" {
		lines = append(lines, "延续上一轮结论："+truncate(previousAnswer, 100))
	}
	if question != "" {
		lines = append(lines, "问题："+question)
	}
	lines = append(lines, "根据检索证据：")
	for i, ctx := range contexts {
		if i >= 3 {
			break
		}
		docName := strings.TrimSpace(ctx.DocName)
		if docName == "" {
			docName = ctx.DocID
		}
		lines = append(lines, fmt.Sprintf("%d. [%s] %s", i+1, docName, truncate(strings.ReplaceAll(strings.TrimSpace(ctx.Content), "\n", " "), 150)))
	}
	lines = append(lines, "结论：请以上述证据为依据继续追问细节。")
	return strings.Join(lines, "\n")
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	var normA float64
	var normB float64
	for i := 0; i < len(a); i++ {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA <= 0 || normB <= 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func truncate(in string, n int) string {
	r := []rune(strings.TrimSpace(in))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var ErrUnavailable = errors.New("chat completer unavailable")
