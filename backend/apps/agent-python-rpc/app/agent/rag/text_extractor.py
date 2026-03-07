from __future__ import annotations

from app.agent.rag.base import BaseDocumentExtractor
from app.agent.rag.common import normalize_text


class TextDocumentExtractor(BaseDocumentExtractor):
    def supports(self, filename: str, mime_type: str) -> bool:
        lower_name = (filename or "").strip().lower()
        lower_mime = (mime_type or "").strip().lower()
        return lower_name.endswith(".txt") or lower_mime.startswith("text/plain")

    def extract(self, filename: str, mime_type: str, content: bytes) -> str:
        # Prefer strict utf-8 and gracefully fallback for mixed-encoding files.
        try:
            text = normalize_text(content.decode("utf-8"))
        except UnicodeDecodeError:
            text = normalize_text(content.decode("utf-8", errors="ignore"))
        if not text:
            raise ValueError("document is empty")
        return text
