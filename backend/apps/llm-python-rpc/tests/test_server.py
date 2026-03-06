import unittest

import grpc

from app.config import Config
from app.server import LlmService
from qa.v1 import qa_pb2


class _DummyContext:
    def abort(self, code, details):
        raise RuntimeError(f"{code}: {details}")


class _FakeClient:
    def __init__(self):
        self.last_messages = None

    def available(self) -> bool:
        return True

    def embed(self, texts, model):
        if len(texts) == 3 and texts[0] == "项目概述是什么":
            return [[1.0, 0.0], [0.9, 0.0], [0.0, 1.0]]
        return [[0.1, 0.2, 0.3] for _ in texts]

    def chat_completion(self, messages, model, temperature):
        self.last_messages = messages
        return "项目概述：智能文档问答助手支持文档上传、检索与多轮问答。"


class LlmServiceTestCase(unittest.TestCase):
    def _build_service(self) -> LlmService:
        cfg = Config(
            host="0.0.0.0",
            port=50051,
            listen_addr="127.0.0.1:50051",
            timeout_seconds=30,
            api_base="https://api.siliconflow.cn/v1",
            api_key="k_test",
            chat_model="Pro/MiniMaxAI/MiniMax-M2.5",
            embedding_model="Qwen/Qwen3-Embedding-4B",
            temperature=0.2,
            default_provider="siliconflow",
            provider_chat_models={"siliconflow": "Pro/MiniMaxAI/MiniMax-M2.5"},
            max_context_chunks=1,
        )
        svc = LlmService(cfg)
        svc.client = _FakeClient()
        return svc

    def test_embed_texts_returns_vectors(self) -> None:
        svc = self._build_service()
        out = svc.EmbedTexts(qa_pb2.EmbedTextsRequest(texts=["a", "b"]), _DummyContext())
        self.assertEqual(len(out.vectors), 2)
        self.assertAlmostEqual(out.vectors[0].values[0], 0.1, places=6)
        self.assertAlmostEqual(out.vectors[0].values[1], 0.2, places=6)
        self.assertAlmostEqual(out.vectors[0].values[2], 0.3, places=6)

    def test_generate_answer_reranks_contexts(self) -> None:
        svc = self._build_service()
        req = qa_pb2.GenerateAnswerRequest(
            question="项目概述是什么",
            scope_type="doc",
            active_provider="siliconflow",
            contexts=[
                qa_pb2.LlmContextChunk(doc_id="d1", doc_name="doc.md", chunk_id="c1", chunk_index=0, content="这是项目概述段落"),
                qa_pb2.LlmContextChunk(doc_id="d1", doc_name="doc.md", chunk_id="c2", chunk_index=1, content="无关内容"),
            ],
        )
        reply = svc.GenerateAnswer(req, _DummyContext())
        self.assertIn("项目概述", reply.answer)
        self.assertIsNotNone(svc.client.last_messages)
        user_content = svc.client.last_messages[1]["content"]
        self.assertIn("[1]", user_content)
        self.assertIn("这是项目概述段落", user_content)

    def test_generate_answer_requires_api_key(self) -> None:
        cfg = Config(
            host="0.0.0.0",
            port=50051,
            listen_addr="127.0.0.1:50051",
            timeout_seconds=30,
            api_base="https://api.siliconflow.cn/v1",
            api_key="",
            chat_model="Pro/MiniMaxAI/MiniMax-M2.5",
            embedding_model="Qwen/Qwen3-Embedding-4B",
            temperature=0.2,
            default_provider="siliconflow",
            provider_chat_models={"siliconflow": "Pro/MiniMaxAI/MiniMax-M2.5"},
            max_context_chunks=1,
        )
        svc = LlmService(cfg)
        with self.assertRaises(RuntimeError) as cm:
            svc.GenerateAnswer(qa_pb2.GenerateAnswerRequest(question="你好"), _DummyContext())
        self.assertIn(str(grpc.StatusCode.FAILED_PRECONDITION), str(cm.exception))


if __name__ == "__main__":
    unittest.main()
