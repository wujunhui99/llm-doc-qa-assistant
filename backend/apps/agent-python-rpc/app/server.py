from __future__ import annotations

import logging
import math
import os
import sys
from concurrent import futures
from pathlib import Path
from typing import List, Sequence

import grpc

from app.agent.llm import BaseChatClient, SiliconFlowClient, build_chat_clients
from app.config import Config
from app.agent.rag import extract_document_text

_PROTO = Path(__file__).resolve().parent / "proto"
if str(_PROTO) not in sys.path:
    sys.path.insert(0, str(_PROTO))

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
        self.chat_clients = build_chat_clients(cfg)
        self.embedding_client = SiliconFlowClient(cfg.api_base, cfg.api_key, cfg.timeout_seconds)
        self.default_provider = (cfg.default_provider or "").strip().lower() or "siliconflow"
        self.logger = logging.getLogger("agent-python-rpc")

    def _resolve_client(self, provider: str) -> tuple[str, BaseChatClient]:
        name = (provider or "").strip().lower() or self.default_provider
        if name == "chatgpt":
            name = "openai"
        elif name == "local":
            name = "ollama"
        client = self.chat_clients.get(name)
        if client is None:
            raise RuntimeError(f"provider is not supported: {name}")
        return name, client

    def Health(self, request: qa_pb2.Empty, context: grpc.ServicerContext) -> qa_pb2.HealthReply:
        return qa_pb2.HealthReply(status="ok", time="")

    def EmbedTexts(self, request: qa_pb2.EmbedTextsRequest, context: grpc.ServicerContext) -> qa_pb2.EmbedTextsReply:
        if not self.embedding_client.available():
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, "embedding provider is not configured: siliconflow")
        try:
            vectors = self.embedding_client.embed(request.texts, self.cfg.embedding_model)
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

    def ExtractDocumentText(
        self, request: qa_pb2.ExtractDocumentTextRequest, context: grpc.ServicerContext
    ) -> qa_pb2.ExtractDocumentTextReply:
        try:
            text = extract_document_text(request.filename, request.mime_type, request.content)
        except ValueError as exc:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(exc))
        except ModuleNotFoundError as exc:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, f"document extraction dependency missing: {exc}")
        except Exception as exc:
            context.abort(grpc.StatusCode.UNAVAILABLE, f"document extraction failed: {exc}")
        return qa_pb2.ExtractDocumentTextReply(text=text)

    def _select_contexts(
        self,
        question: str,
        contexts: List[qa_pb2.LlmContextChunk],
    ) -> List[qa_pb2.LlmContextChunk]:
        if not contexts:
            return []

        limit = max(1, self.cfg.max_context_chunks)
        if len(contexts) <= limit:
            return list(contexts)

        # Use embedding similarity in python service for final context rerank.
        texts = [question] + [c.content for c in contexts]
        try:
            vectors = self.embedding_client.embed(texts, self.cfg.embedding_model)
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

    def _build_messages(
        self,
        scope_type: str,
        prev_q: str,
        prev_a: str,
        question: str,
        selected_contexts: Sequence[qa_pb2.LlmContextChunk],
    ) -> List[dict]:
        if selected_contexts:
            context_lines: List[str] = []
            for idx, c in enumerate(selected_contexts, start=1):
                doc_name = (c.doc_name or "").strip() or (c.doc_id or "")
                context_lines.append(f"[{idx}] {doc_name}#{c.chunk_index}: {_truncate(c.content, 240)}")

            return [
                {
                    "role": "system",
                    "content": (
                        "你是严谨的文档问答助手。只能根据给定证据回答，不允许编造。"
                        "回答中尽量引用证据编号[1][2]...。"
                        "回答应准确、简洁，优先给结论；除非用户明确要求，不要展开冗长分析。"
                        "禁止输出思考过程、推理步骤或过程化措辞（例如“首先/其次/分析如下”），只输出最终答复。"
                    ),
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

        return [
            {
                "role": "system",
                "content": (
                    "你是文档问答助手。当前没有检索到任何文档证据。"
                    "你可以进行简短通用对话，但要明确说明回答不基于文档证据。"
                    "回答应准确、简洁，优先给结论；除非用户明确要求，不要展开冗长分析。"
                    "禁止输出思考过程、推理步骤或过程化措辞（例如“首先/其次/分析如下”），只输出最终答复。"
                ),
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

    def _prepare_generation(
        self, request: qa_pb2.GenerateAnswerRequest
    ) -> tuple[str, BaseChatClient, str, str, List[dict], int, bool]:
        provider = (request.active_provider or "").strip().lower() or self.cfg.default_provider
        resolved_provider, client = self._resolve_client(provider)
        if not client.available():
            raise RuntimeError(f"provider is not configured: {resolved_provider}")

        model = (
            self.cfg.provider_chat_models.get(provider)
            or self.cfg.provider_chat_models.get(resolved_provider)
            or self.cfg.chat_model
        )

        scope_type = _resolve_scope(request.scope_type)
        prev_q = (request.previous_turn_question or "").strip()
        prev_a = (request.previous_turn_answer or "").strip()
        question = (request.question or "").strip()
        selected_contexts = self._select_contexts(question, list(request.contexts))
        messages = self._build_messages(scope_type, prev_q, prev_a, question, selected_contexts)
        # Reasoning mode is globally disabled for stable short outputs.
        think_mode = False
        return resolved_provider, client, model, scope_type, messages, len(selected_contexts), think_mode

    def _abort_llm_error(self, context: grpc.ServicerContext, exc: Exception) -> None:
        msg = str(exc).lower()
        if "api_key" in msg or "api key" in msg or "401" in msg or "unauthorized" in msg:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, f"LLM provider authentication/config error: {exc}")
        context.abort(grpc.StatusCode.UNAVAILABLE, f"LLM provider call failed: {exc}")

    def GenerateAnswer(self, request: qa_pb2.GenerateAnswerRequest, context: grpc.ServicerContext) -> qa_pb2.GenerateAnswerReply:
        try:
            resolved_provider, client, model, scope_type, messages, context_count, think_mode = self._prepare_generation(request)
        except Exception as exc:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, f"LLM provider routing error: {exc}")

        try:
            self.logger.info(
                "generate answer route provider=%s model=%s scope=%s contexts=%d think_mode=%s",
                resolved_provider,
                model,
                scope_type,
                context_count,
                think_mode,
            )
            answer = client.chat_completion(messages, model=model, temperature=self.cfg.temperature, think_mode=think_mode)
        except Exception as exc:
            self._abort_llm_error(context, exc)

        return qa_pb2.GenerateAnswerReply(answer=answer)

    def StreamGenerateAnswer(self, request: qa_pb2.GenerateAnswerRequest, context: grpc.ServicerContext):
        try:
            resolved_provider, client, model, scope_type, messages, context_count, think_mode = self._prepare_generation(request)
        except Exception as exc:
            context.abort(grpc.StatusCode.FAILED_PRECONDITION, f"LLM provider routing error: {exc}")

        try:
            self.logger.info(
                "stream generate answer route provider=%s model=%s scope=%s contexts=%d think_mode=%s",
                resolved_provider,
                model,
                scope_type,
                context_count,
                think_mode,
            )
            chunks: List[str] = []
            for stream_chunk in client.chat_completion_stream(
                messages, model=model, temperature=self.cfg.temperature, think_mode=think_mode
            ):
                piece = ""
                thinking_piece = ""
                if isinstance(stream_chunk, str):
                    piece = stream_chunk
                elif isinstance(stream_chunk, dict):
                    piece = str((stream_chunk or {}).get("delta") or "")
                    thinking_piece = str((stream_chunk or {}).get("thinking_delta") or "")
                elif stream_chunk is not None:
                    piece = str(stream_chunk)
                if not piece and not thinking_piece:
                    continue
                if piece:
                    chunks.append(piece)
                yield qa_pb2.GenerateAnswerChunk(
                    delta=piece,
                    answer="".join(chunks),
                    done=False,
                    thinking_delta=thinking_piece,
                )

            answer = "".join(chunks).strip()
            if not answer:
                raise RuntimeError("LLM provider returned empty answer")
            yield qa_pb2.GenerateAnswerChunk(
                delta="",
                answer=answer,
                done=True,
                thinking_delta="",
            )
        except Exception as exc:
            self._abort_llm_error(context, exc)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="[agent-python-rpc] %(asctime)s %(message)s")
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
        fallback = "unix:///tmp/agent-python-rpc.sock"
        if bind != fallback:
            sock_path = fallback.removeprefix("unix://")
            if sock_path and os.path.exists(sock_path):
                os.remove(sock_path)
            bind = fallback
            server.add_insecure_port(bind)
        else:
            raise
    server.start()
    logging.info("agent grpc listening on %s", bind)
    server.wait_for_termination()


if __name__ == "__main__":
    main()
