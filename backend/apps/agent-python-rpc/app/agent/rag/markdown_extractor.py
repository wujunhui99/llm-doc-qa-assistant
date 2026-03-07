from __future__ import annotations

import re

from app.agent.rag.base import BaseDocumentExtractor
from app.agent.rag.common import normalize_text


_INLINE_CODE = re.compile(r"`([^`]*)`")
_LINK = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")
_HEADING = re.compile(r"^\s{0,3}#{1,6}\s*", flags=re.MULTILINE)
_LIST_MARKER = re.compile(r"^\s*[-*+]\s+", flags=re.MULTILINE)


def _markdown_to_text(raw: str) -> str:
    text = raw
    text = _HEADING.sub("", text)
    text = _LIST_MARKER.sub("", text)
    text = _INLINE_CODE.sub(r"\1", text)
    text = _LINK.sub(r"\1", text)
    return text


class MarkdownDocumentExtractor(BaseDocumentExtractor):
    def supports(self, filename: str, mime_type: str) -> bool:
        lower_name = (filename or "").strip().lower()
        lower_mime = (mime_type or "").strip().lower()
        return lower_name.endswith(".md") or lower_name.endswith(".markdown") or "markdown" in lower_mime

    def extract(self, filename: str, mime_type: str, content: bytes) -> str:
        try:
            raw = content.decode("utf-8")
        except UnicodeDecodeError:
            raw = content.decode("utf-8", errors="ignore")
        text = normalize_text(_markdown_to_text(raw))
        if not text:
            raise ValueError("document is empty")
        return text
