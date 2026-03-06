from __future__ import annotations

import json
import math
from typing import Any

from app.config import AppConfig
from app.llm.siliconflow_client import SiliconFlowClient, SiliconFlowError


class AgentOrchestrator:
    def __init__(self, client: SiliconFlowClient, config: AppConfig) -> None:
        self._client = client
        self._cfg = config

    def generate_answer(self, request, active_provider: str = "") -> str:
        question = (request.question or "").strip()
        previous_question = (request.previous_turn_question or "").strip()
        previous_answer = (request.previous_turn_answer or "").strip()
        contexts = list(request.contexts)

        if not contexts:
            return self._fallback_no_context(previous_answer)

        ranked_contexts = self._rerank_contexts(question, contexts)
        plan = self._build_plan(question, previous_question, ranked_contexts, active_provider)
        selected_contexts = self._execute_tools(plan, ranked_contexts)
        if not selected_contexts:
            selected_contexts = ranked_contexts[: self._cfg.agent.max_context_chunks]

        return self._build_final_answer(
            question=question,
            previous_question=previous_question,
            previous_answer=previous_answer,
            selected_contexts=selected_contexts,
            scope_type=(request.scope_type or "all"),
            active_provider=active_provider,
        )

    def _rerank_contexts(self, question: str, contexts: list[Any]) -> list[Any]:
        limit = self._cfg.agent.max_context_chunks
        if not question:
            return contexts[:limit]
        if not self._client.available:
            return contexts[:limit]

        texts = [(ctx.content or "").strip() for ctx in contexts]
        if not any(texts):
            return contexts[:limit]

        try:
            vectors = self._client.embeddings(
                [question] + texts,
                model=self._cfg.siliconflow.embedding_model,
            )
            if len(vectors) != len(contexts) + 1:
                return contexts[:limit]
        except SiliconFlowError:
            return contexts[:limit]

        q_vec = vectors[0]
        scored: list[tuple[float, Any]] = []
        for idx, ctx in enumerate(contexts, start=1):
            score = _cosine_similarity(q_vec, vectors[idx])
            scored.append((score, ctx))

        scored.sort(key=lambda item: item[0], reverse=True)
        return [ctx for _, ctx in scored[:limit]]

    def _build_plan(self, question: str, previous_question: str, contexts: list[Any], active_provider: str) -> dict[str, Any]:
        default_plan = {
            "keywords": [],
            "max_contexts": min(self._cfg.agent.max_context_chunks, len(contexts)),
            "focus": "",
        }
        if not self._client.available:
            return default_plan

        context_preview = []
        for idx, ctx in enumerate(contexts[: self._cfg.agent.max_context_chunks], start=1):
            excerpt = _truncate((ctx.content or "").replace("\n", " ").strip(), 140)
            context_preview.append(f"{idx}. {ctx.doc_name or ctx.doc_id}#{ctx.chunk_index}: {excerpt}")

        messages = [
            {
                "role": "system",
                "content": (
                    "你是检索问答Agent规划器。"
                    "请输出严格JSON，字段: keywords(字符串数组), max_contexts(整数), focus(字符串)。"
                    "不要输出额外解释。"
                ),
            },
            {
                "role": "user",
                "content": (
                    f"问题: {question}\n"
                    f"上一轮问题: {previous_question}\n"
                    "可用证据:\n"
                    + "\n".join(context_preview)
                ),
            },
        ]

        try:
            raw = self._client.chat_completion(
                messages=messages,
                model=self._resolve_chat_model(active_provider),
                temperature=self._cfg.agent.plan_temperature,
            )
            parsed = _parse_json_object(raw)
            if not parsed:
                return default_plan

            keywords = parsed.get("keywords", [])
            if not isinstance(keywords, list):
                keywords = []
            keywords = [str(k).strip() for k in keywords if str(k).strip()]

            max_contexts = parsed.get("max_contexts", default_plan["max_contexts"])
            try:
                max_contexts = int(max_contexts)
            except Exception:
                max_contexts = default_plan["max_contexts"]
            max_contexts = max(1, min(max_contexts, self._cfg.agent.max_context_chunks, len(contexts)))

            focus = str(parsed.get("focus", "")).strip()
            return {
                "keywords": keywords,
                "max_contexts": max_contexts,
                "focus": focus,
            }
        except SiliconFlowError:
            return default_plan

    def _execute_tools(self, plan: dict[str, Any], contexts: list[Any]) -> list[Any]:
        max_contexts = int(plan.get("max_contexts", self._cfg.agent.max_context_chunks))
        max_contexts = max(1, min(max_contexts, self._cfg.agent.max_context_chunks))
        keywords = [k.lower() for k in plan.get("keywords", []) if isinstance(k, str) and k.strip()]

        selected: list[Any] = []
        if keywords:
            for ctx in contexts:
                content = (ctx.content or "").lower()
                if any(keyword in content for keyword in keywords):
                    selected.append(ctx)
                    if len(selected) >= max_contexts:
                        break

        if len(selected) < max_contexts:
            for ctx in contexts:
                if ctx in selected:
                    continue
                selected.append(ctx)
                if len(selected) >= max_contexts:
                    break

        return selected

    def _build_final_answer(
        self,
        question: str,
        previous_question: str,
        previous_answer: str,
        selected_contexts: list[Any],
        scope_type: str,
        active_provider: str,
    ) -> str:
        if not self._client.available:
            return self._fallback_answer(question, previous_answer, selected_contexts)

        context_lines = []
        for idx, ctx in enumerate(selected_contexts, start=1):
            excerpt = _truncate((ctx.content or "").replace("\n", " ").strip(), 240)
            context_lines.append(f"[{idx}] {ctx.doc_name or ctx.doc_id}#{ctx.chunk_index}: {excerpt}")

        messages = [
            {
                "role": "system",
                "content": (
                    "你是严谨的文档问答助手。"
                    "只能根据给定证据回答，不允许编造。"
                    "回答中尽量引用证据编号[1][2]...，并保持中文表达清晰连贯。"
                ),
            },
            {
                "role": "user",
                "content": (
                    f"作用域: {scope_type}\n"
                    f"上一轮问题: {previous_question}\n"
                    f"上一轮答案: {previous_answer}\n"
                    f"当前问题: {question}\n"
                    "证据:\n"
                    + "\n".join(context_lines)
                ),
            },
        ]

        try:
            answer = self._client.chat_completion(
                messages=messages,
                model=self._resolve_chat_model(active_provider),
                temperature=self._cfg.agent.answer_temperature,
            )
            answer = answer.strip()
            if answer:
                return answer
        except SiliconFlowError:
            pass

        return self._fallback_answer(question, previous_answer, selected_contexts)

    def _resolve_chat_model(self, active_provider: str) -> str:
        provider_key = (active_provider or self._cfg.provider or "siliconflow").strip().lower()
        if provider_key in self._cfg.siliconflow.provider_chat_models:
            return self._cfg.siliconflow.provider_chat_models[provider_key]
        return self._cfg.siliconflow.chat_model

    def _fallback_no_context(self, previous_answer: str) -> str:
        if previous_answer:
            return (
                "未检索到新证据，先延续上一轮结论："
                f"{_truncate(previous_answer, 140)}。"
                "建议切换到 @all 或补充更具体关键词。"
            )
        return "当前作用域未检索到足够证据，请切换作用域或补充关键词后重试。"

    def _fallback_answer(self, question: str, previous_answer: str, selected_contexts: list[Any]) -> str:
        lines: list[str] = []
        if previous_answer:
            lines.append(f"延续上一轮结论：{_truncate(previous_answer, 100)}")
        if question:
            lines.append(f"问题：{question}")
        lines.append("根据检索证据：")
        for idx, ctx in enumerate(selected_contexts[:3], start=1):
            lines.append(
                f"{idx}. [{ctx.doc_name or ctx.doc_id}] {_truncate((ctx.content or '').replace(chr(10), ' ').strip(), 150)}"
            )
        lines.append("结论：请以上述证据为依据继续追问细节。")
        return "\n".join(lines)


def _truncate(text: str, n: int) -> str:
    if len(text) <= n:
        return text
    return text[:n] + "..."


def _parse_json_object(raw: str) -> dict[str, Any] | None:
    text = (raw or "").strip()
    if not text:
        return None

    # Strip markdown code fences if present.
    if text.startswith("```"):
        parts = text.split("```")
        for part in parts:
            part = part.strip()
            if part.startswith("{") and part.endswith("}"):
                text = part
                break

    start = text.find("{")
    end = text.rfind("}")
    if start >= 0 and end > start:
        text = text[start : end + 1]

    try:
        parsed = json.loads(text)
    except Exception:
        return None
    if not isinstance(parsed, dict):
        return None
    return parsed


def _cosine_similarity(a: list[float], b: list[float]) -> float:
    if not a or not b or len(a) != len(b):
        return 0.0
    dot = 0.0
    norm_a = 0.0
    norm_b = 0.0
    for i in range(len(a)):
        dot += a[i] * b[i]
        norm_a += a[i] * a[i]
        norm_b += b[i] * b[i]
    if norm_a <= 0.0 or norm_b <= 0.0:
        return 0.0
    return dot / (math.sqrt(norm_a) * math.sqrt(norm_b))
