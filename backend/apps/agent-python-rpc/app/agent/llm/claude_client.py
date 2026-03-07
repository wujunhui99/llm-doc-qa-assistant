from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, Iterator, Sequence

from app.agent.llm.base import BaseChatClient
from app.agent.llm.litellm_utils import (
    build_model_name,
    iter_stream_chunks,
    litellm_completion,
    normalize_messages,
    parse_completion_response,
)


@dataclass
class ClaudeClient(BaseChatClient):
    api_base: str
    api_key: str
    timeout_seconds: int
    provider: str = "claude"

    def available(self) -> bool:
        return bool((self.api_key or "").strip())

    def _timeout_seconds(self) -> int:
        try:
            timeout = int(self.timeout_seconds)
        except (TypeError, ValueError):
            timeout = 30
        return max(1, timeout)

    def _base_payload(self, messages: Sequence[dict], model: str, temperature: float) -> dict:
        return {
            "model": build_model_name("anthropic", model),
            "messages": normalize_messages(messages),
            "temperature": float(temperature),
            "timeout": self._timeout_seconds(),
            "api_base": (self.api_base or "").strip(),
            "api_key": (self.api_key or "").strip(),
        }

    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False) -> str:
        _ = think_mode
        if not self.available():
            raise RuntimeError("ANTHROPIC_API_KEY is empty")

        payload = self._base_payload(messages, model, temperature)
        try:
            resp = litellm_completion(payload)
        except Exception as exc:
            raise RuntimeError(f"claude chat request failed: {exc}") from exc
        answer, _ = parse_completion_response(resp)
        if not answer:
            raise RuntimeError("claude chat completion returned empty answer")
        return answer

    def chat_completion_stream(
        self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False
    ) -> Iterator[Dict[str, str]]:
        _ = think_mode
        if not self.available():
            raise RuntimeError("ANTHROPIC_API_KEY is empty")
        payload = self._base_payload(messages, model, temperature)
        payload["stream"] = True
        try:
            stream = litellm_completion(payload)
        except Exception as exc:
            raise RuntimeError(f"claude chat request failed: {exc}") from exc
        emitted = False
        for chunk in iter_stream_chunks(stream):
            emitted = True
            yield chunk
        if not emitted:
            raise RuntimeError("claude chat completion returned empty answer")

    def chat_completion_with_tools(
        self,
        messages: Sequence[dict],
        model: str,
        temperature: float,
        tools: Sequence[dict],
        tool_choice: str = "auto",
        think_mode: bool = False,
    ) -> Dict[str, object]:
        _ = think_mode
        if not self.available():
            raise RuntimeError("ANTHROPIC_API_KEY is empty")
        payload = self._base_payload(messages, model, temperature)
        payload["tools"] = list(tools)
        payload["tool_choice"] = tool_choice
        try:
            resp = litellm_completion(payload)
        except Exception as exc:
            raise RuntimeError(f"claude chat request failed: {exc}") from exc
        content, tool_calls = parse_completion_response(resp)
        if not content and not tool_calls:
            raise RuntimeError("claude chat tool-call response is empty")
        return {"content": content, "tool_calls": tool_calls}
