from __future__ import annotations

from dataclasses import dataclass
import json
from typing import Iterator, Sequence

import requests
from requests import Response

from app.agent.llm.base import BaseChatClient


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

    def _post_chat(self, payload: dict, timeout: int, stream: bool = False) -> Response:
        return requests.post(
            f"{self._base_url()}/api/chat",
            json=payload,
            timeout=timeout,
            stream=stream,
        )

    def chat_completion(self, messages: Sequence[dict], model: str, temperature: float) -> str:
        if not self.available():
            raise RuntimeError("OLLAMA_API_BASE is empty")

        payload = {
            "model": model,
            "messages": list(messages),
            "stream": False,
            "keep_alive": "10m",
            "options": {"temperature": float(temperature)},
        }
        timeout = self._timeout_seconds()
        try:
            resp = self._post_chat(payload, timeout=timeout, stream=False)
        except requests.exceptions.ReadTimeout:
            retry_timeout = max(timeout * 2, 180)
            try:
                resp = self._post_chat(payload, timeout=retry_timeout, stream=False)
            except requests.exceptions.ReadTimeout as exc:
                raise RuntimeError(
                    "ollama chat request timeout, consider increasing OLLAMA_TIMEOUT_SECONDS: "
                    f"{exc}"
                ) from exc
            except requests.RequestException as exc:
                raise RuntimeError(f"ollama chat request failed: {exc}") from exc
        except requests.RequestException as exc:
            raise RuntimeError(f"ollama chat request failed: {exc}") from exc

        if resp.status_code < 200 or resp.status_code >= 300:
            raise RuntimeError(f"ollama chat api status={resp.status_code} body={resp.text}")

        try:
            data = resp.json()
        except ValueError as exc:
            raise RuntimeError(f"ollama chat api returned invalid json: {exc}") from exc

        if not isinstance(data, dict):
            raise RuntimeError("ollama chat api returned invalid response body")

        content = ((data.get("message") or {}).get("content") or "")
        content = str(content).strip()
        if not content:
            raise RuntimeError("ollama chat completion returned empty answer")
        return content

    def _stream_chat_once(self, payload: dict, timeout: int) -> Iterator[str]:
        resp = self._post_chat(payload, timeout=timeout, stream=True)
        try:
            if resp.status_code < 200 or resp.status_code >= 300:
                raise RuntimeError(f"ollama chat api status={resp.status_code} body={resp.text}")

            for line in resp.iter_lines(decode_unicode=True):
                if not line:
                    continue
                try:
                    data = json.loads(line)
                except ValueError as exc:
                    raise RuntimeError(f"ollama chat stream returned invalid json: {exc}") from exc
                if not isinstance(data, dict):
                    raise RuntimeError("ollama chat stream returned invalid response body")

                delta = str(((data.get("message") or {}).get("content") or ""))
                if delta:
                    yield delta
                if bool(data.get("done")):
                    break
        finally:
            resp.close()

    def chat_completion_stream(self, messages: Sequence[dict], model: str, temperature: float) -> Iterator[str]:
        if not self.available():
            raise RuntimeError("OLLAMA_API_BASE is empty")

        payload = {
            "model": model,
            "messages": list(messages),
            "stream": True,
            "keep_alive": "10m",
            "options": {"temperature": float(temperature)},
        }
        timeout = self._timeout_seconds()
        timeouts = [timeout, max(timeout * 2, 180)]
        last_exc: Exception | None = None

        for idx, current_timeout in enumerate(timeouts):
            emitted = False
            try:
                for delta in self._stream_chat_once(payload, current_timeout):
                    emitted = True
                    yield delta
                if emitted:
                    return
                raise RuntimeError("ollama chat completion returned empty answer")
            except requests.exceptions.ReadTimeout as exc:
                last_exc = exc
                if emitted or idx == len(timeouts) - 1:
                    raise RuntimeError(
                        "ollama chat stream timeout, consider increasing OLLAMA_TIMEOUT_SECONDS: "
                        f"{exc}"
                    ) from exc
                continue
            except requests.RequestException as exc:
                raise RuntimeError(f"ollama chat request failed: {exc}") from exc

        raise RuntimeError(f"ollama chat stream failed: {last_exc}")
