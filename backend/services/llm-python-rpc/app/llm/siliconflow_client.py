from __future__ import annotations

import json
import urllib.error
import urllib.request
from typing import Any

from app.config import SiliconFlowConfig


class SiliconFlowError(Exception):
    pass


class SiliconFlowClient:
    def __init__(self, config: SiliconFlowConfig) -> None:
        self._cfg = config

    @property
    def available(self) -> bool:
        return bool(self._cfg.api_key.strip())

    def chat_completion(
        self,
        messages: list[dict[str, str]],
        model: str,
        temperature: float,
    ) -> str:
        if not self.available:
            raise SiliconFlowError("SILICONFLOW_API_KEY is empty")

        payload = {
            "model": model,
            "messages": messages,
            "temperature": temperature,
        }
        data = self._post_json("/chat/completions", payload)

        try:
            content = data["choices"][0]["message"]["content"]
            if not isinstance(content, str):
                raise TypeError("content is not string")
            return content.strip()
        except Exception as exc:  # pragma: no cover - defensive parsing
            raise SiliconFlowError(f"unexpected chat response format: {exc}") from exc

    def embeddings(self, inputs: list[str], model: str) -> list[list[float]]:
        if not self.available:
            raise SiliconFlowError("SILICONFLOW_API_KEY is empty")
        if not inputs:
            return []

        payload = {
            "model": model,
            "input": inputs,
        }
        data = self._post_json("/embeddings", payload)

        rows = data.get("data")
        if not isinstance(rows, list):
            raise SiliconFlowError("unexpected embeddings response: missing data")

        rows_sorted = sorted(rows, key=lambda item: item.get("index", 0))
        vectors: list[list[float]] = []
        for row in rows_sorted:
            embedding = row.get("embedding")
            if not isinstance(embedding, list):
                raise SiliconFlowError("unexpected embeddings response: invalid embedding")
            vectors.append([float(x) for x in embedding])
        return vectors

    def _post_json(self, path: str, payload: dict[str, Any]) -> dict[str, Any]:
        url = self._build_url(path)
        body = json.dumps(payload).encode("utf-8")
        req = urllib.request.Request(
            url=url,
            data=body,
            method="POST",
            headers={
                "Authorization": f"Bearer {self._cfg.api_key}",
                "Content-Type": "application/json",
                "Accept": "application/json",
            },
        )

        try:
            with urllib.request.urlopen(req, timeout=self._cfg.timeout_seconds) as resp:
                raw = resp.read().decode("utf-8")
        except urllib.error.HTTPError as err:
            detail = err.read().decode("utf-8", errors="ignore")
            raise SiliconFlowError(f"siliconflow http error {err.code}: {detail}") from err
        except urllib.error.URLError as err:
            raise SiliconFlowError(f"siliconflow request failed: {err}") from err

        try:
            parsed = json.loads(raw)
        except Exception as exc:
            raise SiliconFlowError(f"invalid json response: {exc}") from exc

        if isinstance(parsed, dict) and parsed.get("error"):
            raise SiliconFlowError(f"siliconflow error: {parsed.get('error')}")

        if not isinstance(parsed, dict):
            raise SiliconFlowError("unexpected response type")

        return parsed

    def _build_url(self, path: str) -> str:
        base = self._cfg.api_base.rstrip("/")
        suffix = "/" + path.lstrip("/")
        return base + suffix
