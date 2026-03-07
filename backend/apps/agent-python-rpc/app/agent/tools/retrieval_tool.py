from __future__ import annotations

import json
from typing import Any, Dict, Sequence

ROUTER_SCOPE = "route"


def retrieval_tool_definition() -> Dict[str, Any]:
    return {
        "type": "function",
        "function": {
            "name": "retrieval",
            "description": "Use document retrieval before answering, provide concise retrieval keywords.",
            "parameters": {
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Short retrieval keywords or rewritten query for semantic search.",
                    },
                    "reason": {
                        "type": "string",
                        "description": "Why retrieval is needed.",
                    },
                },
                "required": ["query"],
                "additionalProperties": False,
            },
        },
    }


def build_router_messages(question: str, prev_q: str, prev_a: str) -> list[dict]:
    q = (question or "").strip()
    return [
        {
            "role": "system",
            "content": (
                "You are a retrieval router.\n"
                "If document retrieval is needed, call function `retrieval` with a concise `query`.\n"
                "If retrieval is not needed, do NOT call tools. Reply with compact JSON only: "
                '{"use_retrieval":false,"reason":"...","retrieval_query":""}.'
            ),
        },
        {
            "role": "user",
            "content": (
                f"Previous question: {(prev_q or '').strip()}\n"
                f"Previous answer: {(prev_a or '').strip()}\n"
                f"Current question: {q}\n"
            ),
        },
    ]


def _parse_json_text(text: str) -> Dict[str, Any]:
    raw = (text or "").strip()
    if not raw:
        return {}
    start = raw.find("{")
    end = raw.rfind("}")
    if start < 0 or end <= start:
        return {}
    try:
        out = json.loads(raw[start : end + 1])
        if isinstance(out, dict):
            return out
    except Exception:
        return {}
    return {}


def parse_router_output(content: str, tool_calls: Sequence[Dict[str, Any]] | None) -> Dict[str, Any]:
    if tool_calls:
        for call in tool_calls:
            if not isinstance(call, dict):
                continue
            name = str(call.get("name") or "").strip().lower()
            if name != "retrieval":
                continue
            args_raw = call.get("arguments")
            args: Dict[str, Any] = {}
            if isinstance(args_raw, str):
                try:
                    loaded = json.loads(args_raw)
                    if isinstance(loaded, dict):
                        args = loaded
                except Exception:
                    args = {}
            query = str(args.get("query") or "").strip()
            reason = str(args.get("reason") or "").strip() or "tool: retrieval"
            return {
                "use_retrieval": True,
                "reason": reason,
                "retrieval_query": query,
                "tool": "retrieval",
                "arguments": {"query": query},
            }

    row = _parse_json_text(content)
    if row:
        use = bool(row.get("use_retrieval"))
        reason = str(row.get("reason") or "").strip() or "json decision"
        query = str(row.get("retrieval_query") or "").strip()
        if not query and isinstance(row.get("arguments"), dict):
            query = str((row.get("arguments") or {}).get("query") or "").strip()
        tool = str(row.get("tool") or "").strip()
        if tool.lower() == "retrieval" and not use:
            use = True
        return {
            "use_retrieval": use,
            "reason": reason,
            "retrieval_query": query,
            "tool": tool,
            "arguments": {"query": query},
        }

    lower = (content or "").lower()
    if "skip retrieval" in lower or "不需要检索" in lower or "use_retrieval\":false" in lower:
        return {"use_retrieval": False, "reason": "text decision", "retrieval_query": "", "tool": "", "arguments": {}}
    if "use retrieval" in lower or "需要检索" in lower or "use_retrieval\":true" in lower:
        return {"use_retrieval": True, "reason": "text decision", "retrieval_query": "", "tool": "", "arguments": {}}

    return {"use_retrieval": True, "reason": "fallback decision", "retrieval_query": "", "tool": "", "arguments": {}}
