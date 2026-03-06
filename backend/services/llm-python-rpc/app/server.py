from __future__ import annotations

import os
import sys
from concurrent import futures
from datetime import datetime, timezone
from pathlib import Path

import grpc

APP_DIR = Path(__file__).resolve().parent
GENERATED_DIR = APP_DIR / "generated"
if str(GENERATED_DIR) not in sys.path:
    sys.path.insert(0, str(GENERATED_DIR))

from qa.v1 import qa_pb2, qa_pb2_grpc  # noqa: E402


class LlmService(qa_pb2_grpc.LlmServiceServicer):
    def __init__(self, internal_service_token: str) -> None:
        self._internal_service_token = internal_service_token.strip()

    def Health(self, request, context):  # noqa: N802
        _ = request
        return qa_pb2.HealthReply(status="ok", time=utc_now())

    def GenerateAnswer(self, request, context):  # noqa: N802
        if self._internal_service_token:
            md = {k: v for k, v in context.invocation_metadata()}
            if md.get("x-service-token", "") != self._internal_service_token:
                context.abort(grpc.StatusCode.PERMISSION_DENIED, "invalid internal service token")

        question = request.question.strip()
        contexts = list(request.contexts)
        answer = compose_answer(
            question=question,
            contexts=contexts,
            previous_question=request.previous_turn_question,
            previous_answer=request.previous_turn_answer,
            scope_type=request.scope_type,
        )
        return qa_pb2.GenerateAnswerReply(answer=answer)


def compose_answer(
    question: str,
    contexts: list,
    previous_question: str,
    previous_answer: str,
    scope_type: str,
) -> str:
    question = (question or "").strip()
    previous_question = (previous_question or "").strip()
    previous_answer = (previous_answer or "").strip()

    if not contexts:
        if previous_answer:
            return (
                "未检索到新证据，先延续上一轮结论："
                f"{truncate(previous_answer, 140)}。"
                "建议切换到 @all 或补充更具体关键词。"
            )
        return "当前作用域未检索到足够证据，请切换作用域或补充关键词后重试。"

    lines = []
    if previous_question:
        lines.append(f"承接上一轮问题“{truncate(previous_question, 32)}”，")
    if question:
        lines.append(f"针对你的问题“{question}”，基于{scope_type or 'all'}作用域证据：")
    else:
        lines.append("基于当前作用域证据：")

    for idx, chunk in enumerate(contexts[:3], start=1):
        excerpt = truncate((chunk.content or "").replace("\n", " ").strip(), 170)
        doc_name = chunk.doc_name or chunk.doc_id or "unknown"
        lines.append(f"{idx}. [{doc_name}] {excerpt}")

    lines.append("结论：请以上述引用作为依据继续追问细节。")
    return "\n".join(lines)


def truncate(text: str, n: int) -> str:
    text = (text or "").strip()
    if len(text) <= n:
        return text
    return text[:n] + "..."


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def normalize_bind_addr(raw: str) -> str:
    raw = (raw or "").strip()
    if not raw:
        return "0.0.0.0:19091"
    if raw.startswith(":"):
        return "0.0.0.0" + raw
    return raw


def serve() -> None:
    bind_addr = normalize_bind_addr(os.getenv("LLM_RPC_ADDR", "0.0.0.0:19091"))
    internal_service_token = os.getenv("INTERNAL_SERVICE_TOKEN", "")

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    qa_pb2_grpc.add_LlmServiceServicer_to_server(LlmService(internal_service_token), server)
    server.add_insecure_port(bind_addr)
    server.start()

    print(f"[llm-python-rpc] listening on {bind_addr}")
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
