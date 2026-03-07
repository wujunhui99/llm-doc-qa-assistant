from __future__ import annotations

from dataclasses import dataclass
from typing import Iterable, List, Sequence

import requests

from app.agent.llm.base import BaseChatClient

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

    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float) -> str:
        if not self.available():
            raise RuntimeError("SILICONFLOW_API_KEY is empty")

        payload = {
            "model": model,
            "messages": list(messages),
            "temperature": temperature,
        }
        resp = requests.post(
            f"{self.api_base.rstrip('/')}/chat/completions",
            headers=self._headers(),
            json=payload,
            timeout=self.timeout_seconds,
        )
        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"chat api status={resp.status_code} body={resp.text}")

        data = resp.json()
        choices = data.get("choices") or []
        if not choices:
            raise RuntimeError("invalid chat response: choices is empty")
        content = ((choices[0] or {}).get("message") or {}).get("content") or ""
        content = str(content).strip()
        if not content:
            raise RuntimeError("chat completion returned empty answer")
        return content

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
