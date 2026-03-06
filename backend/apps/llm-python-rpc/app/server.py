from __future__ import annotations

import logging
import math
import os
import sys
from concurrent import futures
from pathlib import Path
from typing import List

import grpc

from app.config import Config
from app.llm.siliconflow_client import SiliconFlowClient

_GENERATED = Path(__file__).resolve().parent / "generated"
if str(_GENERATED) not in sys.path:
    sys.path.insert(0, str(_GENERATED))

from qa.v1 import qa_pb2, qa_pb2_grpc


def _resolve_scope(scope_type: str) -> str:
    scope = (scope_type or "").strip()
    return scope or "all"


def _truncate(text: str, n: int) -> str:
    text = (text or "").strip().replace("\n", " ")
    if len(text) <= n:
        return text
    return text[:n] + "..."


def _cosine(a: List[float], b: List[float]) -> float:
    if not a or not b or len(a) != len(b):
        return 0.0
    dot = 0.0
    norm_a = 0.0
    norm_b = 0.0
    for x, y in zip(a, b):
        dot += float(x) * float(y)
        norm_a += float(x) * float(x)
        norm_b += float(y) * float(y)
    if norm_a <= 0 or norm_b <= 0:
        return 0.0
    return dot / (math.sqrt(norm_a) * math.sqrt(norm_b))


class LlmService(qa_pb2_grpc.LlmServiceServicer):
    def __init__(self, cfg: Config):
        self.cfg = cfg
        self.client = SiliconFlowClient(cfg.api_base, cfg.api_key, cfg.timeout_seconds)
        self.logger = logging.getLogger("llm-python-rpc")

    def Health(self, request: qa_pb2.Empty, context: grpc.ServicerContext) -> qa_pb2.HealthReply:
        return qa_pb2.HealthReply(status="ok", time="")

    def EmbedTexts(self, request: qa_pb2.EmbedTextsRequest, context: grpc.ServicerContext) -> qa_pb2.EmbedTextsReply:
        try:
            vectors = self.client.embed(request.texts, self.cfg.embedding_model)
        except Exception as exc:
            msg = str(exc).lower()
            if "api_key" in msg or "api key" in msg or "401" in msg or "unauthorized" in msg:
                context.abort(grpc.StatusCode.FAILED_PRECONDITION, f"LLM embedding authentication/config error: {exc}")
            if "empty" in msg and "api_key" in msg:
                context.abort(grpc.StatusCode.FAILED_PRECONDITION, f"LLM embedding unavailable: {exc}")
            context.abort(grpc.StatusCode.UNAVAILABLE, f"LLM embedding call failed: {exc}")

        return qa_pb2.EmbedTextsReply(
            vectors=[qa_pb2.EmbeddingVector(values=v) for v in vectors]
        )

    def _select_contexts(self, question: str, contexts: List[qa_pb2.LlmContextChunk]) -> List[qa_pb2.LlmContextChunk]:
        if not contexts:
            return []

        limit = max(1, self.cfg.max_context_chunks)
        if len(contexts) <= limit:
            return list(contexts)

        # Use embedding similarity in python service for final context rerank.
        texts = [question] + [c.content for c in contexts]
        try:
            vectors = self.client.embed(texts, self.cfg.embedding_model)
        except Exception as exc:
            self.logger.warning("context rerank embedding failed, fallback top-N: %s", exc)
            return list(contexts[:limit])

        if len(vectors) != len(texts):
            return list(contexts[:limit])

        qvec = vectors[0]
        scored = []
        for idx, ctx in enumerate(contexts):
            score = _cosine(qvec, vectors[idx + 1])
            scored.append((score, idx, ctx))
        scored.sort(key=lambda item: (-item[0], item[1]))
        return [item[2] for item in scored[:limit]]

    def GenerateAnswer(self, request: qa_pb2.GenerateAnswerRequest, context: grpc.ServicerContext) -> qa_pb2.GenerateAnswerReply:
        if not self.client.available():
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "SILICONFLOW_API_KEY is empty")

        provider = (request.active_provider or "").strip().lower() or self.cfg.default_provider
        model = self.cfg.provider_chat_models.get(provider) or self.cfg.chat_model

        scope_type = _resolve_scope(request.scope_type)
        prev_q = (request.previous_turn_question or "").strip()
        prev_a = (request.previous_turn_answer or "").strip()
        question = (request.question or "").strip()

        selected_contexts = self._select_contexts(question, list(request.contexts))

        if selected_contexts:
            context_lines: List[str] = []
            for idx, c in enumerate(selected_contexts, start=1):
                doc_name = (c.doc_name or "").strip() or (c.doc_id or "")
                context_lines.append(f"[{idx}] {doc_name}#{c.chunk_index}: {_truncate(c.content, 240)}")

            messages = [
                {
                    "role": "system",
                    "content": "你是严谨的文档问答助手。只能根据给定证据回答，不允许编造。回答中尽量引用证据编号[1][2]...，并保持中文表达清晰连贯。",
                },
                {
                    "role": "user",
                    "content": (
                        f"作用域: {scope_type}\n"
                        f"上一轮问题: {prev_q}\n"
                        f"上一轮答案: {prev_a}\n"
                        f"当前问题: {question}\n"
                        f"证据:\n" + "\n".join(context_lines)
                    ),
                },
            ]
        else:
            messages = [
                {
                    "role": "system",
                    "content": "你是文档问答助手。当前没有检索到任何文档证据。你可以进行简短通用对话，但要明确说明回答不基于文档证据。",
                },
                {
                    "role": "user",
                    "content": (
                        f"作用域: {scope_type}\n"
                        f"上一轮问题: {prev_q}\n"
                        f"上一轮答案: {prev_a}\n"
                        f"当前问题: {question}\n"
                        f"当前证据: 无"
                    ),
                },
            ]

        try:
            answer = self.client.chat_completion(messages, model=model, temperature=self.cfg.temperature)
        except Exception as exc:
            msg = str(exc).lower()
            if "api_key" in msg or "api key" in msg or "401" in msg or "unauthorized" in msg:
                context.abort(grpc.StatusCode.FAILED_PRECONDITION, f"LLM provider authentication/config error: {exc}")
            context.abort(grpc.StatusCode.UNAVAILABLE, f"LLM provider call failed: {exc}")

        return qa_pb2.GenerateAnswerReply(answer=answer)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="[llm-python-rpc] %(asctime)s %(message)s")
    cfg = Config.load()

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    qa_pb2_grpc.add_LlmServiceServicer_to_server(LlmService(cfg), server)
    bind = cfg.listen_addr
    if bind.startswith("unix://"):
        sock_path = bind.removeprefix("unix://")
        if sock_path and os.path.exists(sock_path):
            os.remove(sock_path)
    try:
        server.add_insecure_port(bind)
    except RuntimeError:
        fallback = "unix:///tmp/llm-python-rpc.sock"
        if bind != fallback:
            sock_path = fallback.removeprefix("unix://")
            if sock_path and os.path.exists(sock_path):
                os.remove(sock_path)
            bind = fallback
            server.add_insecure_port(bind)
        else:
            raise
    server.start()
    logging.info("llm grpc listening on %s", bind)
    server.wait_for_termination()


if __name__ == "__main__":
    main()
