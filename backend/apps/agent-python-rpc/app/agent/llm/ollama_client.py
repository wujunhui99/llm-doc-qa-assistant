from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, Iterator, Sequence

from app.agent.llm.base import BaseChatClient
from app.agent.llm.litellm_utils import (
    build_model_name,
    is_timeout_error,
    iter_stream_chunks,
    litellm_completion,
    normalize_messages,
    parse_completion_response,
)


@dataclass
class OllamaClient(BaseChatClient):
    api_base: str
    timeout_seconds: int
    provider: str = "ollama"

    def available(self) -> bool:
        return bool((self.api_base or "").strip())

    def _base_url(self) -> str:
        return (self.api_base or "").strip().rstrip("/")

    def _timeout_seconds(self) -> int:
        try:
            timeout = int(self.timeout_seconds)
        except (TypeError, ValueError):
            timeout = 30
        return max(1, timeout)

    def _base_payload(self, messages: Sequence[dict], model: str, temperature: float, timeout: int, stream: bool) -> dict:
        return {
            "model": build_model_name("ollama", model),
            "messages": normalize_messages(messages),
            "temperature": float(temperature),
            "stream": bool(stream),
            "timeout": timeout,
            "api_base": self._base_url(),
            "num_predict": 2048,
            "think": bool(False),
        }

    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False) -> str:
        if not self.available():
            raise RuntimeError("OLLAMA_API_BASE is empty")

        timeout = self._timeout_seconds()
        timeouts = [timeout, max(timeout * 2, 180)]
        last_exc: Exception | None = None
        for idx, current_timeout in enumerate(timeouts):
            payload = self._base_payload(
                messages=messages,
                model=model,
                temperature=temperature,
                timeout=current_timeout,
                stream=False,
            )
            payload["think"] = bool(think_mode)
            try:
                resp = litellm_completion(payload)
                answer, _ = parse_completion_response(resp)
                if not answer:
                    raise RuntimeError("ollama chat completion returned empty answer")
                return answer
            except Exception as exc:
                last_exc = exc
                if is_timeout_error(exc):
                    if idx == len(timeouts) - 1:
                        raise RuntimeError(
                            "ollama chat request timeout, consider increasing OLLAMA_TIMEOUT_SECONDS: "
                            f"{exc}"
                        ) from exc
                    continue
                raise RuntimeError(f"ollama chat request failed: {exc}") from exc
        raise RuntimeError(f"ollama chat request failed: {last_exc}")

    def chat_completion_stream(
        self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False
    ) -> Iterator[Dict[str, str]]:
        if not self.available():
            raise RuntimeError("OLLAMA_API_BASE is empty")

        timeout = self._timeout_seconds()
        timeouts = [timeout, max(timeout * 2, 180)]
        last_exc: Exception | None = None

        for idx, current_timeout in enumerate(timeouts):
            emitted = False
            payload = self._base_payload(
                messages=messages,
                model=model,
                temperature=temperature,
                timeout=current_timeout,
                stream=True,
            )
            payload["think"] = bool(think_mode)
            try:
                stream = litellm_completion(payload)
                for chunk in iter_stream_chunks(stream):
                    if not think_mode and chunk.get("thinking_delta"):
                        chunk = {"delta": chunk.get("delta", ""), "thinking_delta": ""}
                    if not chunk.get("delta") and not chunk.get("thinking_delta"):
                        continue
                    emitted = True
                    yield chunk
                if emitted:
                    return
                raise RuntimeError("ollama chat completion returned empty answer")
            except Exception as exc:
                last_exc = exc
                if not is_timeout_error(exc):
                    raise RuntimeError(f"ollama chat request failed: {exc}") from exc
                if emitted or idx == len(timeouts) - 1:
                    raise RuntimeError(
                        "ollama chat stream timeout, consider increasing OLLAMA_TIMEOUT_SECONDS: "
                        f"{exc}"
                    ) from exc
                continue

        raise RuntimeError(f"ollama chat stream failed: {last_exc}")
