from __future__ import annotations

from dataclasses import dataclass
from typing import Dict, Iterable, Iterator, List, Sequence

import requests

from app.agent.llm.base import BaseChatClient
from app.agent.llm.litellm_utils import (
    build_model_name,
    iter_stream_chunks,
    litellm_completion,
    normalize_messages,
    parse_completion_response,
)

@dataclass
class SiliconFlowClient(BaseChatClient):
    api_base: str
    api_key: str
    timeout_seconds: int
    provider: str = "siliconflow"

    def available(self) -> bool:
        return bool(self.api_key.strip())

    def _headers(self) -> dict:
        return {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        }

    def _timeout_seconds(self) -> int:
        try:
            timeout = int(self.timeout_seconds)
        except (TypeError, ValueError):
            timeout = 30
        return max(1, timeout)

    def _base_payload(self, messages: Sequence[dict], model: str, temperature: float) -> dict:
        return {
            "model": build_model_name("openai", model),
            "messages": normalize_messages(messages),
            "temperature": float(temperature),
            "timeout": self._timeout_seconds(),
            "api_base": (self.api_base or "").strip(),
            "api_key": (self.api_key or "").strip(),
        }

    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False) -> str:
        _ = think_mode
        if not self.available():
            raise RuntimeError("SILICONFLOW_API_KEY is empty")

        payload = self._base_payload(messages, model, temperature)
        try:
            resp = litellm_completion(payload)
        except Exception as exc:
            raise RuntimeError(f"chat request failed: {exc}") from exc
        content, _ = parse_completion_response(resp)
        if not content:
            raise RuntimeError("chat completion returned empty answer")
        return content

    def chat_completion_stream(
        self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False
    ) -> Iterator[Dict[str, str]]:
        _ = think_mode
        if not self.available():
            raise RuntimeError("SILICONFLOW_API_KEY is empty")

        payload = self._base_payload(messages, model, temperature)
        payload["stream"] = True
        try:
            stream = litellm_completion(payload)
        except Exception as exc:
            raise RuntimeError(f"chat request failed: {exc}") from exc

        emitted = False
        for chunk in iter_stream_chunks(stream):
            emitted = True
            yield chunk
        if not emitted:
            raise RuntimeError("chat completion returned empty answer")

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
            raise RuntimeError("SILICONFLOW_API_KEY is empty")
        payload = self._base_payload(messages, model, temperature)
        payload["tools"] = list(tools)
        payload["tool_choice"] = tool_choice
        try:
            resp = litellm_completion(payload)
        except Exception as exc:
            raise RuntimeError(f"chat request failed: {exc}") from exc
        content, tool_calls = parse_completion_response(resp)
        if not content and not tool_calls:
            raise RuntimeError("chat tool-call response is empty")
        return {"content": content, "tool_calls": tool_calls}

    def embed(self, texts: Iterable[str], model: str) -> List[List[float]]:
        if not self.available():
            raise RuntimeError("SILICONFLOW_API_KEY is empty")

        items = [str(t).strip() for t in texts]
        if not items:
            return []

        payload = {
            "model": model,
            "input": items,
        }
        resp = requests.post(
            f"{self.api_base.rstrip('/')}/embeddings",
            headers=self._headers(),
            json=payload,
            timeout=self.timeout_seconds,
        )
        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"embeddings api status={resp.status_code} body={resp.text}")

        data = resp.json()
        rows = data.get("data") or []
        vectors: List[List[float]] = []
        for row in rows:
            emb = row.get("embedding") if isinstance(row, dict) else None
            if not isinstance(emb, list):
                continue
            vectors.append([float(x) for x in emb])

        if len(vectors) != len(items):
            raise RuntimeError(f"embeddings count mismatch: want={len(items)} got={len(vectors)}")
        return vectors
