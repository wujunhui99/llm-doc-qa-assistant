package logic

import (
	"context"
	"time"

	qav1 "llm-doc-qa-assistant/backend/proto/gen/go/qa/v1"
	"llm-doc-qa-assistant/backend/apps/api-go/internal/svc"
)

type DocumentLogic struct {
	svcCtx *svc.ServiceContext
}

func NewDocumentLogic(svcCtx *svc.ServiceContext) *DocumentLogic {
	return &DocumentLogic{svcCtx: svcCtx}
}

func (l *DocumentLogic) ListDocuments(ctx context.Context, token string, page, pageSize int32) (*qav1.ListDocumentsReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	return l.svcCtx.Core.ListDocuments(ctx, &qav1.ListDocumentsRequest{Token: token, Page: page, PageSize: pageSize})
}

func (l *DocumentLogic) UploadDocument(ctx context.Context, token, filename, mimeType string, content []byte) (*qav1.DocumentReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	return l.svcCtx.Core.UploadDocument(ctx, &qav1.UploadDocumentRequest{Token: token, Filename: filename, MimeType: mimeType, Content: content})
}

func (l *DocumentLogic) GetDocument(ctx context.Context, token, docID string) (*qav1.DocumentReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	return l.svcCtx.Core.GetDocument(ctx, &qav1.DocumentRequest{Token: token, DocumentId: docID})
}

func (l *DocumentLogic) DownloadDocument(ctx context.Context, token, docID string) (*qav1.DownloadDocumentReply, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return l.svcCtx.Core.DownloadDocument(ctx, &qav1.DocumentRequest{Token: token, DocumentId: docID})
}

func (l *DocumentLogic) DeleteDocument(ctx context.Context, token, docID string, confirm bool) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	_, err := l.svcCtx.Core.DeleteDocument(ctx, &qav1.DeleteDocumentRequest{Token: token, DocumentId: docID, Confirm: confirm})
	return err
}
