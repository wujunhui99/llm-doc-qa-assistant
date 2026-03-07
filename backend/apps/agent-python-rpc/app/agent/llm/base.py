from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Iterator
from typing import Sequence


class BaseChatClient(ABC):
    provider: str

    @abstractmethod
    def available(self) -> bool:
        raise NotImplementedError

    @abstractmethod
    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float) -> str:
        raise NotImplementedError

    def chat_completion_stream(self, messages: Sequence[dict], model: str, temperature: float) -> Iterator[str]:
        # Default fallback for providers without native streaming support.
        answer = self.chat_completion(messages, model, temperature)
        if answer:
            yield answer
