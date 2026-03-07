from __future__ import annotations

from typing import Iterable

from app.agent.rag.base import BaseDocumentExtractor
from app.agent.rag.markdown_extractor import MarkdownDocumentExtractor
from app.agent.rag.pdf_extractor import PDFDocumentExtractor
from app.agent.rag.text_extractor import TextDocumentExtractor


class DocumentExtractionService:
    def __init__(self, extractors: Iterable[BaseDocumentExtractor] | None = None):
        # Order matters: more specific extractors should come first.
        self.extractors = list(extractors or [PDFDocumentExtractor(), MarkdownDocumentExtractor(), TextDocumentExtractor()])

    def extract_document_text(self, filename: str, mime_type: str, content: bytes) -> str:
        for extractor in self.extractors:
            if extractor.supports(filename, mime_type):
                return extractor.extract(filename, mime_type, content)
        raise ValueError("unsupported document type")


_DEFAULT_SERVICE = DocumentExtractionService()


def extract_document_text(filename: str, mime_type: str, content: bytes) -> str:
    return _DEFAULT_SERVICE.extract_document_text(filename, mime_type, content)
