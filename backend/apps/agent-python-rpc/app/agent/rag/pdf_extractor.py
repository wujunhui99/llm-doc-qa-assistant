from __future__ import annotations

import io

from pypdf import PdfReader

from app.agent.rag.base import BaseDocumentExtractor
from app.agent.rag.common import is_readable_text, normalize_text


class PDFDocumentExtractor(BaseDocumentExtractor):
    def supports(self, filename: str, mime_type: str) -> bool:
        lower_name = (filename or "").strip().lower()
        lower_mime = (mime_type or "").strip().lower()
        return lower_name.endswith(".pdf") or "pdf" in lower_mime

    def extract(self, filename: str, mime_type: str, content: bytes) -> str:
        reader = PdfReader(io.BytesIO(content))
        parts: list[str] = []
        for page in reader.pages:
            raw = page.extract_text() or ""
            cleaned = normalize_text(raw)
            if cleaned:
                parts.append(cleaned)

        text = normalize_text("\n".join(parts))
        if not text:
            raise ValueError("unable to extract text from pdf")
        if not is_readable_text(text):
            raise ValueError("unable to extract readable text from pdf")
        return text
