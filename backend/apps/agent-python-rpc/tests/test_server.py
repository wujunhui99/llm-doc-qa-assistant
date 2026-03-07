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
        self.last_think_mode = None

    def available(self) -> bool:
        return True

    def chat_completion(self, messages, model, temperature, think_mode=False):
        self.last_messages = messages
        self.last_think_mode = think_mode
        return "项目概述：智能文档问答助手支持文档上传、检索与多轮问答。"

    def chat_completion_stream(self, messages, model, temperature, think_mode=False):
        self.last_messages = messages
        self.last_think_mode = think_mode
        yield {"delta": "", "thinking_delta": "先分析用户问题"}
        yield {"delta": "项目概述：", "thinking_delta": ""}
        yield {"delta": "智能文档问答助手支持文档上传、检索与多轮问答。", "thinking_delta": ""}

class _FakeEmbeddingClient:
    def available(self) -> bool:
        return True

    def embed(self, texts, model):
        if len(texts) == 3 and texts[0] == "项目概述是什么":
            return [[1.0, 0.0], [0.9, 0.0], [0.0, 1.0]]
        return [[0.1, 0.2, 0.3] for _ in texts]


class LlmServiceTestCase(unittest.TestCase):
    def _build_service(self) -> LlmService:
        cfg = Config(
            host="0.0.0.0",
            port=50051,
            listen_addr="127.0.0.1:50051",
            timeout_seconds=30,
            ollama_timeout_seconds=180,
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
        fake = _FakeClient()
        svc.chat_clients = {"siliconflow": fake, "openai": fake}
        svc.embedding_client = _FakeEmbeddingClient()
        svc.default_provider = "siliconflow"
        svc._fake_client = fake
        return svc

    def test_embed_texts_returns_vectors(self) -> None:
        svc = self._build_service()
        out = svc.EmbedTexts(qa_pb2.EmbedTextsRequest(texts=["a", "b"]), _DummyContext())
        self.assertEqual(len(out.vectors), 2)
        self.assertAlmostEqual(out.vectors[0].values[0], 0.1, places=6)
        self.assertAlmostEqual(out.vectors[0].values[1], 0.2, places=6)
        self.assertAlmostEqual(out.vectors[0].values[2], 0.3, places=6)

    def test_extract_document_text_markdown(self) -> None:
        svc = self._build_service()
        out = svc.ExtractDocumentText(
            qa_pb2.ExtractDocumentTextRequest(
                filename="spec.md",
                mime_type="text/markdown",
                content="项⽬概述\n系统支持文档问答。".encode("utf-8"),
            ),
            _DummyContext(),
        )
        self.assertEqual(out.text, "项目概述 系统支持文档问答。")

    def test_extract_document_text_rejects_unsupported(self) -> None:
        svc = self._build_service()
        with self.assertRaises(RuntimeError) as cm:
            svc.ExtractDocumentText(
                qa_pb2.ExtractDocumentTextRequest(
                    filename="a.exe",
                    mime_type="application/octet-stream",
                    content=b"abc",
                ),
                _DummyContext(),
            )
        self.assertIn(str(grpc.StatusCode.INVALID_ARGUMENT), str(cm.exception))

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
        self.assertIsNotNone(svc._fake_client.last_messages)
        user_content = svc._fake_client.last_messages[1]["content"]
        self.assertIn("[1]", user_content)
        self.assertIn("这是项目概述段落", user_content)
        self.assertEqual(svc._fake_client.last_think_mode, False)

    def test_generate_answer_requires_api_key(self) -> None:
        cfg = Config(
            host="0.0.0.0",
            port=50051,
            listen_addr="127.0.0.1:50051",
            timeout_seconds=30,
            ollama_timeout_seconds=180,
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

    def test_generate_answer_rejects_unknown_provider(self) -> None:
        svc = self._build_service()
        with self.assertRaises(RuntimeError) as cm:
            svc.GenerateAnswer(
                qa_pb2.GenerateAnswerRequest(
                    question="你好",
                    active_provider="unknown-provider",
                ),
                _DummyContext(),
            )
        self.assertIn(str(grpc.StatusCode.FAILED_PRECONDITION), str(cm.exception))

    def test_generate_answer_chatgpt_alias_uses_openai_client(self) -> None:
        svc = self._build_service()
        reply = svc.GenerateAnswer(
            qa_pb2.GenerateAnswerRequest(
                question="项目概述是什么",
                active_provider="chatgpt",
                contexts=[qa_pb2.LlmContextChunk(doc_id="d1", chunk_id="c1", content="项目概述段落")],
            ),
            _DummyContext(),
        )
        self.assertIn("项目概述", reply.answer)

    def test_stream_generate_answer_returns_chunks(self) -> None:
        svc = self._build_service()
        chunks = list(
            svc.StreamGenerateAnswer(
                qa_pb2.GenerateAnswerRequest(
                    question="项目概述是什么",
                    active_provider="siliconflow",
                    think_mode=True,
                ),
                _DummyContext(),
            )
        )
        self.assertGreaterEqual(len(chunks), 2)
        self.assertEqual(chunks[-1].done, True)
        self.assertIn("项目概述", chunks[-1].answer)
        self.assertEqual(svc._fake_client.last_think_mode, True)


if __name__ == "__main__":
    unittest.main()
