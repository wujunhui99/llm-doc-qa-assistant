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

from app.config import AppConfig, load_config  # noqa: E402
from app.llm.agent_orchestrator import AgentOrchestrator  # noqa: E402
from app.llm.siliconflow_client import SiliconFlowClient  # noqa: E402


class LlmService(qa_pb2_grpc.LlmServiceServicer):
    def __init__(self, config: AppConfig, orchestrator: AgentOrchestrator, internal_service_token: str) -> None:
        self._config = config
        self._orchestrator = orchestrator
        self._internal_service_token = internal_service_token.strip()

    def Health(self, request, context):  # noqa: N802
        _ = request
        return qa_pb2.HealthReply(status="ok", time=utc_now())

    def GenerateAnswer(self, request, context):  # noqa: N802
        md = {k: v for k, v in context.invocation_metadata()}
        if self._internal_service_token:
            if md.get("x-service-token", "") != self._internal_service_token:
                context.abort(grpc.StatusCode.PERMISSION_DENIED, "invalid internal service token")

        active_provider = (md.get("x-active-provider", "") or self._config.provider).strip()
        answer = self._orchestrator.generate_answer(request=request, active_provider=active_provider)
        return qa_pb2.GenerateAnswerReply(answer=answer)


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
    config = load_config()
    bind_addr = normalize_bind_addr(os.getenv("LLM_RPC_ADDR", "0.0.0.0:19091"))
    internal_service_token = os.getenv("INTERNAL_SERVICE_TOKEN", "")

    client = SiliconFlowClient(config.siliconflow)
    orchestrator = AgentOrchestrator(client=client, config=config)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    qa_pb2_grpc.add_LlmServiceServicer_to_server(
        LlmService(config=config, orchestrator=orchestrator, internal_service_token=internal_service_token),
        server,
    )
    server.add_insecure_port(bind_addr)
    server.start()

    print(f"[llm-python-rpc] listening on {bind_addr}")
    print(
        "[llm-python-rpc] provider=siliconflow "
        f"chat_model={config.siliconflow.chat_model} "
        f"embedding_model={config.siliconflow.embedding_model}"
    )
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
