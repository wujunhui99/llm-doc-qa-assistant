from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from app.config import AgentConfig, AppConfig, SiliconFlowConfig
from app.llm.agent_orchestrator import AgentOrchestrator


class _Ctx:
    def __init__(self, doc_id: str, doc_name: str, chunk_id: str, chunk_index: int, content: str):
        self.doc_id = doc_id
        self.doc_name = doc_name
        self.chunk_id = chunk_id
        self.chunk_index = chunk_index
        self.content = content


class _Req:
    def __init__(self, question: str, contexts: list[_Ctx]):
        self.question = question
        self.contexts = contexts
        self.previous_turn_question = ""
        self.previous_turn_answer = ""
        self.scope_type = "all"


class _FakeClient:
    def __init__(self, available: bool, vectors: list[list[float]] | None = None):
        self.available = available
        self._vectors = vectors or []

    def embeddings(self, inputs, model):
        _ = (inputs, model)
        return self._vectors

    def chat_completion(self, messages, model, temperature):
        _ = (messages, model, temperature)
        return '{"keywords": ["延期"], "max_contexts": 1, "focus": "交付时间"}'


class _SequenceClient:
    def __init__(self, vectors: list[list[float]]) -> None:
        self.available = True
        self._vectors = vectors
        self._chat_calls = 0

    def embeddings(self, inputs, model):
        _ = (inputs, model)
        return self._vectors

    def chat_completion(self, messages, model, temperature):
        _ = (messages, model, temperature)
        self._chat_calls += 1
        if self._chat_calls == 1:
            return '{"keywords": ["延期", "交付"], "max_contexts": 2, "focus": "交付时间"}'
        return "根据证据[1]，项目整体延期2周，新的交付日期为2026-04-15。"


class AgentOrchestratorTests(unittest.TestCase):
    def _cfg(self) -> AppConfig:
        sf = SiliconFlowConfig(
            api_base="https://api.siliconflow.cn/v1",
            api_key="x",
            chat_model="Pro/MiniMaxAI/MiniMax-M2.5",
            embedding_model="Qwen/Qwen3-Embedding-4B",
            timeout_seconds=30,
            temperature=0.2,
            provider_chat_models={"siliconflow": "Pro/MiniMaxAI/MiniMax-M2.5", "mock": "Pro/MiniMaxAI/MiniMax-M2.5"},
        )
        agent = AgentConfig(max_context_chunks=3, max_steps=2, plan_temperature=0.0, answer_temperature=0.2)
        return AppConfig(provider="siliconflow", siliconflow=sf, agent=agent)

    def test_generate_answer_fallback_when_client_unavailable(self) -> None:
        cfg = self._cfg()
        client = _FakeClient(available=False)
        orchestrator = AgentOrchestrator(client=client, config=cfg)

        req = _Req(
            question="延期多久",
            contexts=[_Ctx("d1", "合同", "c1", 0, "项目延期2周")],
        )
        out = orchestrator.generate_answer(req, active_provider="")
        self.assertIn("根据检索证据", out)

    def test_rerank_contexts_by_embedding_similarity(self) -> None:
        cfg = self._cfg()
        vectors = [
            [1.0, 0.0],
            [0.9, 0.1],
            [0.1, 0.9],
        ]
        client = _FakeClient(available=True, vectors=vectors)
        orchestrator = AgentOrchestrator(client=client, config=cfg)

        req = _Req(
            question="延期",
            contexts=[
                _Ctx("d1", "合同A", "c1", 0, "延期说明在这里"),
                _Ctx("d2", "合同B", "c2", 1, "付款说明"),
            ],
        )
        ranked = orchestrator._rerank_contexts(req.question, req.contexts)  # noqa: SLF001
        self.assertEqual(ranked[0].chunk_id, "c1")

    def test_rag_agent_dialogue_with_generated_document(self) -> None:
        cfg = self._cfg()
        with tempfile.TemporaryDirectory() as tmp:
            doc_path = Path(tmp) / "generated_rag_doc.md"
            doc_path.write_text(
                "\n".join(
                    [
                        "# 供应商交付说明",
                        "项目代号 Phoenix。",
                        "原计划 2026-04-01 交付，现因联调问题延期2周，新的日期是 2026-04-15。",
                        "付款节点：验收通过后10个工作日支付尾款。",
                    ]
                ),
                encoding="utf-8",
            )
            content = doc_path.read_text(encoding="utf-8")

        # Simulate retrieval chunks generated from this document.
        contexts = [
            _Ctx("doc_1", "generated_rag_doc.md", "c1", 0, "原计划 2026-04-01 交付。"),
            _Ctx("doc_1", "generated_rag_doc.md", "c2", 1, "因联调问题延期2周，新的日期是 2026-04-15。"),
            _Ctx("doc_1", "generated_rag_doc.md", "c3", 2, "付款节点：验收通过后10个工作日支付尾款。"),
        ]
        self.assertIn("延期2周", content)

        # vectors: [question, c1, c2, c3], c2 should be closest to question.
        vectors = [
            [1.0, 0.0],
            [0.3, 0.7],
            [0.95, 0.05],
            [0.2, 0.8],
        ]
        client = _SequenceClient(vectors=vectors)
        orchestrator = AgentOrchestrator(client=client, config=cfg)

        req = _Req(question="这个项目最终延期多久？", contexts=contexts)
        req.previous_turn_question = "这个项目什么时候交付？"
        req.previous_turn_answer = "原计划是 2026-04-01。"

        answer = orchestrator.generate_answer(req, active_provider="siliconflow")
        self.assertIn("延期2周", answer)
        self.assertIn("2026-04-15", answer)


if __name__ == "__main__":
    unittest.main()
