package llmrpc

import (
	"context"
	"fmt"
	"io"
	"strings"

	corerpc "llm-doc-qa-assistant/backend/apps/core-go-rpc/internal/rpc"
	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"

	"google.golang.org/grpc/status"
)

type Client struct {
	raw qav1.LlmServiceClient
}

func New(raw qav1.LlmServiceClient) *Client {
	return &Client{raw: raw}
}

func (c *Client) GenerateAnswer(ctx context.Context, req corerpc.LLMGenerateRequest) (string, error) {
	if c == nil || c.raw == nil {
		return "", fmt.Errorf("llm rpc client is not configured")
	}

	contexts := make([]*qav1.LlmContextChunk, 0, len(req.Contexts))
	for _, chunk := range req.Contexts {
		contexts = append(contexts, &qav1.LlmContextChunk{
			DocId:      chunk.DocID,
			DocName:    chunk.DocName,
			ChunkId:    chunk.ChunkID,
			ChunkIndex: int32(chunk.ChunkIndex),
			Score:      int32(chunk.Score),
			Content:    chunk.Content,
		})
	}

	resp, err := c.raw.GenerateAnswer(ctx, &qav1.GenerateAnswerRequest{
		OwnerUserId:          req.OwnerUserID,
		ThreadId:             req.ThreadID,
		TurnId:               req.TurnID,
		Question:             req.Question,
		ScopeType:            req.ScopeType,
		ScopeDocIds:          append([]string(nil), req.ScopeDocIDs...),
		Contexts:             contexts,
		PreviousTurnQuestion: req.PreviousTurnQuestion,
		PreviousTurnAnswer:   req.PreviousTurnAnswer,
		ActiveProvider:       req.ActiveProvider,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			return "", fmt.Errorf("%s: %s", strings.ToLower(st.Code().String()), st.Message())
		}
		return "", err
	}

	answer := strings.TrimSpace(resp.GetAnswer())
	if answer == "" {
		return "", fmt.Errorf("llm rpc returned empty answer")
	}
	return answer, nil
}

func (c *Client) GenerateAnswerStream(ctx context.Context, req corerpc.LLMGenerateRequest, onDelta func(string) error) (string, error) {
	if c == nil || c.raw == nil {
		return "", fmt.Errorf("llm rpc client is not configured")
	}

	contexts := make([]*qav1.LlmContextChunk, 0, len(req.Contexts))
	for _, chunk := range req.Contexts {
		contexts = append(contexts, &qav1.LlmContextChunk{
			DocId:      chunk.DocID,
			DocName:    chunk.DocName,
			ChunkId:    chunk.ChunkID,
			ChunkIndex: int32(chunk.ChunkIndex),
			Score:      int32(chunk.Score),
			Content:    chunk.Content,
		})
	}

	stream, err := c.raw.StreamGenerateAnswer(ctx, &qav1.GenerateAnswerRequest{
		OwnerUserId:          req.OwnerUserID,
		ThreadId:             req.ThreadID,
		TurnId:               req.TurnID,
		Question:             req.Question,
		ScopeType:            req.ScopeType,
		ScopeDocIds:          append([]string(nil), req.ScopeDocIDs...),
		Contexts:             contexts,
		PreviousTurnQuestion: req.PreviousTurnQuestion,
		PreviousTurnAnswer:   req.PreviousTurnAnswer,
		ActiveProvider:       req.ActiveProvider,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			return "", fmt.Errorf("%s: %s", strings.ToLower(st.Code().String()), st.Message())
		}
		return "", err
	}

	var builder strings.Builder
	finalAnswer := ""
	for {
		chunk, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			if st, ok := status.FromError(recvErr); ok {
				return "", fmt.Errorf("%s: %s", strings.ToLower(st.Code().String()), st.Message())
			}
			return "", recvErr
		}
		delta := chunk.GetDelta()
		if delta != "" {
			builder.WriteString(delta)
			if onDelta != nil {
				if cbErr := onDelta(delta); cbErr != nil {
					return "", cbErr
				}
			}
		}
		if chunk.GetDone() {
			finalAnswer = strings.TrimSpace(chunk.GetAnswer())
		}
	}

	if strings.TrimSpace(finalAnswer) == "" {
		finalAnswer = strings.TrimSpace(builder.String())
	}
	if finalAnswer == "" {
		return "", fmt.Errorf("llm rpc returned empty answer")
	}
	return finalAnswer, nil
}

func (c *Client) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if c == nil || c.raw == nil {
		return nil, fmt.Errorf("llm rpc client is not configured")
	}
	if len(texts) == 0 {
		return nil, nil
	}

	resp, err := c.raw.EmbedTexts(ctx, &qav1.EmbedTextsRequest{
		Texts: append([]string(nil), texts...),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			return nil, fmt.Errorf("%s: %s", strings.ToLower(st.Code().String()), st.Message())
		}
		return nil, err
	}

	vectors := make([][]float32, 0, len(resp.GetVectors()))
	for _, row := range resp.GetVectors() {
		values := row.GetValues()
		if len(values) == 0 {
			vectors = append(vectors, nil)
			continue
		}
		vec := make([]float32, len(values))
		copy(vec, values)
		vectors = append(vectors, vec)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("vectorization count mismatch: texts=%d vectors=%d", len(texts), len(vectors))
	}
	return vectors, nil
}

func (c *Client) ExtractDocumentText(ctx context.Context, filename, mimeType string, content []byte) (string, error) {
	if c == nil || c.raw == nil {
		return "", fmt.Errorf("llm rpc client is not configured")
	}

	resp, err := c.raw.ExtractDocumentText(ctx, &qav1.ExtractDocumentTextRequest{
		Filename: filename,
		MimeType: mimeType,
		Content:  append([]byte(nil), content...),
	})
	if err != nil {
		if st, ok := status.FromError(err); ok {
			return "", fmt.Errorf("%s: %s", strings.ToLower(st.Code().String()), st.Message())
		}
		return "", err
	}

	text := strings.TrimSpace(resp.GetText())
	if text == "" {
		return "", fmt.Errorf("llm rpc returned empty extracted text")
	}
	return text, nil
}
