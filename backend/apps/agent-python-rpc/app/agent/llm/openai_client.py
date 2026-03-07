from __future__ import annotations

from dataclasses import dataclass
import json
from typing import Dict, Iterator, Sequence

import requests
from requests import Response
from app.agent.llm.base import BaseChatClient


@dataclass
class OpenAIClient(BaseChatClient):
    api_base: str
    api_key: str
    timeout_seconds: int
    provider: str = "openai"

    def available(self) -> bool:
        return bool((self.api_base or "").strip() and (self.api_key or "").strip())

    def _base_url(self) -> str:
        return (self.api_base or "").strip().rstrip("/")

    def _timeout_seconds(self) -> int:
        try:
            timeout = int(self.timeout_seconds)
        except (TypeError, ValueError):
            timeout = 30
        return max(1, timeout)

    def _headers(self) -> dict:
        return {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        }

    def _post_chat(self, payload: dict, timeout: int, stream: bool = False) -> Response:
        return requests.post(
            f"{self._base_url()}/chat/completions",
            headers=self._headers(),
            json=payload,
            timeout=timeout,
            stream=stream,
        )

    @staticmethod
    def _normalize_messages(messages: Sequence[dict]) -> list[dict]:
        out: list[dict] = []
        for row in messages:
            if not isinstance(row, dict):
                continue
            role = str(row.get("role") or "").strip() or "user"
            content = row.get("content")
            if content is None:
                text = ""
            elif isinstance(content, str):
                text = content
            else:
                text = str(content)
            out.append({"role": role, "content": text})
        return out

    @staticmethod
    def _extract_content(content: object) -> str:
        if isinstance(content, str):
            return content
        if isinstance(content, list):
            parts: list[str] = []
            for row in content:
                if not isinstance(row, dict):
                    continue
                txt = row.get("text")
                if isinstance(txt, str) and txt:
                    parts.append(txt)
            return "".join(parts)
        if content is None:
            return ""
        return str(content)

    @classmethod
    def _extract_delta(cls, choice: Dict[str, object]) -> str:
        delta = (choice or {}).get("delta") or {}
        if not isinstance(delta, dict):
            return ""
        return cls._extract_content(delta.get("content"))

    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False) -> str:
        _ = think_mode
        if not self.available():
            raise RuntimeError("OPENAI_API_KEY or OPENAI_API_BASE is empty")

        payload = {
            "model": model,
            "messages": self._normalize_messages(messages),
            "temperature": float(temperature),
        }
        timeout = self._timeout_seconds()
        try:
            resp = self._post_chat(payload, timeout=timeout, stream=False)
        except requests.RequestException as exc:
            raise RuntimeError(f"openai chat request failed: {exc}") from exc

        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"openai chat api status={resp.status_code} body={resp.text}")

        try:
            data = resp.json()
        except ValueError as exc:
            raise RuntimeError(f"openai chat api returned invalid json: {exc}") from exc
        if not isinstance(data, dict):
            raise RuntimeError("openai chat api returned invalid response body")

        choices = data.get("choices") or []
        if not choices:
            raise RuntimeError("openai chat response choices is empty")
        first = choices[0] if isinstance(choices[0], dict) else {}
        message = first.get("message") if isinstance(first.get("message"), dict) else {}
        answer = self._extract_content(message.get("content")).strip()
        if not answer:
            raise RuntimeError("openai chat completion returned empty answer")
        return answer

    def chat_completion_stream(
        self, messages: Sequence[dict], model: str, temperature: float, think_mode: bool = False
    ) -> Iterator[Dict[str, str]]:
        _ = think_mode
        if not self.available():
            raise RuntimeError("OPENAI_API_KEY or OPENAI_API_BASE is empty")

        payload = {
            "model": model,
            "messages": self._normalize_messages(messages),
            "temperature": float(temperature),
            "stream": True,
        }
        timeout = self._timeout_seconds()
        try:
            resp = self._post_chat(payload, timeout=timeout, stream=True)
        except requests.RequestException as exc:
            raise RuntimeError(f"openai chat request failed: {exc}") from exc

        emitted = False
        try:
            if resp.status_code < 200 or resp.status_code >= 300:
                raise RuntimeError(f"openai chat api status={resp.status_code} body={resp.text}")

            for line in resp.iter_lines(decode_unicode=True):
                if not line:
                    continue
                raw = str(line).strip()
                if not raw.startswith("data:"):
                    continue
                payload_text = raw[len("data:") :].strip()
                if not payload_text:
                    continue
                if payload_text == "[DONE]":
                    break
                try:
                    row = json.loads(payload_text)
                except ValueError as exc:
                    raise RuntimeError(f"openai chat stream returned invalid json: {exc}") from exc
                if not isinstance(row, dict):
                    raise RuntimeError("openai chat stream returned invalid response body")
                choices = row.get("choices") or []
                if not choices:
                    continue
                first = choices[0] if isinstance(choices[0], dict) else {}
                delta = self._extract_delta(first)
                if not delta:
                    continue
                emitted = True
                yield {"delta": delta, "thinking_delta": ""}
        finally:
            resp.close()

        if not emitted:
            raise RuntimeError("openai chat completion returned empty answer")
