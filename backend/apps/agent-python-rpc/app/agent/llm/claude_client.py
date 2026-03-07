from __future__ import annotations

from dataclasses import dataclass
from typing import Sequence

from app.agent.llm.base import BaseChatClient


@dataclass
class ClaudeClient(BaseChatClient):
    api_base: str
    api_key: str
    timeout_seconds: int
    provider: str = "claude"

    def available(self) -> bool:
        # Reserved implementation for future integration.
        return bool(self.api_key.strip())

    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False) -> str:
        raise RuntimeError("claude provider is reserved but not implemented yet")
