from __future__ import annotations

from typing import Any, Dict, Iterator, Sequence

import litellm


def _to_dict(value: Any) -> Dict[str, Any]:
    if isinstance(value, dict):
        return value
    if hasattr(value, "model_dump"):
        try:
            dumped = value.model_dump()
            if isinstance(dumped, dict):
                return dumped
        except Exception:
            return {}
    if hasattr(value, "dict"):
        try:
            dumped = value.dict()
            if isinstance(dumped, dict):
                return dumped
        except Exception:
            return {}
    return {}


def normalize_messages(messages: Sequence[dict]) -> list[dict]:
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


def extract_content(content: Any) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for row in content:
            if isinstance(row, dict):
                txt = row.get("text")
                if isinstance(txt, str):
                    parts.append(txt)
                continue
            txt = getattr(row, "text", None)
            if isinstance(txt, str):
                parts.append(txt)
        return "".join(parts)
    if content is None:
        return ""
    return str(content)


def extract_tool_calls(tool_calls: Any) -> list[dict]:
    if not isinstance(tool_calls, list):
        return []
    out: list[dict] = []
    for row in tool_calls:
        item = _to_dict(row)
        if not item and not hasattr(row, "function"):
            continue
        fn = item.get("function")
        if not isinstance(fn, dict):
            fn = _to_dict(getattr(row, "function", None))
        call_id = str(item.get("id") or getattr(row, "id", "") or "").strip()
        name = str((fn or {}).get("name") or "").strip()
        arguments = str((fn or {}).get("arguments") or "").strip()
        out.append({"id": call_id, "name": name, "arguments": arguments})
    return out


def parse_completion_response(resp: Any) -> tuple[str, list[dict]]:
    body = _to_dict(resp)
    choices = getattr(resp, "choices", None)
    if not isinstance(choices, list):
        choices = body.get("choices")
    if not isinstance(choices, list) or not choices:
        raise RuntimeError("chat response choices is empty")

    first = choices[0]
    row = _to_dict(first)
    message = row.get("message")
    if not isinstance(message, dict):
        message = _to_dict(getattr(first, "message", None))

    content = extract_content(message.get("content")).strip()
    tool_calls = extract_tool_calls(message.get("tool_calls"))
    return content, tool_calls


def iter_stream_chunks(stream: Any) -> Iterator[Dict[str, str]]:
    for chunk in stream:
        body = _to_dict(chunk)
        choices = getattr(chunk, "choices", None)
        if not isinstance(choices, list):
            choices = body.get("choices")
        if not isinstance(choices, list) or not choices:
            continue

        first = choices[0]
        row = _to_dict(first)
        delta = row.get("delta")
        if not isinstance(delta, dict):
            delta = _to_dict(getattr(first, "delta", None))

        text = extract_content(delta.get("content"))
        thinking = extract_content(
            delta.get("reasoning_content")
            or delta.get("thinking")
            or delta.get("reasoning")
        )
        if text or thinking:
            yield {"delta": text, "thinking_delta": thinking}


def build_model_name(prefix: str, model: str) -> str:
    raw = (model or "").strip()
    if not raw:
        return raw
    lower = raw.lower()
    if "/" in lower:
        return raw
    clean_prefix = (prefix or "").strip()
    if not clean_prefix:
        return raw
    return f"{clean_prefix}/{raw}"


def is_timeout_error(exc: Exception) -> bool:
    text = str(exc).lower()
    return "timeout" in text or "timed out" in text or "read timeout" in text


def litellm_completion(payload: Dict[str, Any]) -> Any:
    # Keep compatibility across providers by ignoring unsupported optional params.
    litellm.drop_params = True
    return litellm.completion(**payload)
