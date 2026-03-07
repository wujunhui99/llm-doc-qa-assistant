from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Dict, Iterator, Sequence


class BaseChatClient(ABC):
    provider: str

    @abstractmethod
    def available(self) -> bool:
        raise NotImplementedError

    @abstractmethod
    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False) -> str:
        raise NotImplementedError

    def chat_completion_stream(
        self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False
    ) -> Iterator[Dict[str, str]]:
        # Default fallback for providers without native streaming support.
        answer = self.chat_completion(messages, model, temperature, think_mode=think_mode)
        if answer:
            yield {"delta": answer, "thinking_delta": ""}

    def chat_completion_with_tools(
        self,
        messages: Sequence[dict],
        model: str,
        temperature: float,
        tools: Sequence[dict],
        tool_choice: str = "auto",
        think_mode: bool = False,
    ) -> Dict[str, object]:
        _ = tools
        _ = tool_choice
        answer = self.chat_completion(messages, model, temperature, think_mode=think_mode)
        return {"content": answer, "tool_calls": []}
