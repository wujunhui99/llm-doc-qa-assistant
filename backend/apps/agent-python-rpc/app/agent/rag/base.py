from __future__ import annotations

from abc import ABC, abstractmethod


class BaseDocumentExtractor(ABC):
    @abstractmethod
    def supports(self, filename: str, mime_type: str) -> bool:
        raise NotImplementedError

    @abstractmethod
    def extract(self, filename: str, mime_type: str, content: bytes) -> str:
        raise NotImplementedError
