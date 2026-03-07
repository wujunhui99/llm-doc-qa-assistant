import os
import unittest
from unittest.mock import patch

from app.config import Config


class ConfigTestCase(unittest.TestCase):
    def test_load_prefers_env_values(self) -> None:
        env = {
            "SILICONFLOW_API_KEY": "k_test",
            "SILICONFLOW_CHAT_MODEL": "Pro/MiniMaxAI/MiniMax-M2.5",
            "SILICONFLOW_EMBEDDING_MODEL": "Qwen/Qwen3-Embedding-4B",
            "OPENAI_CHAT_MODEL": "gpt-4o",
            "LLM_AGENT_MAX_CONTEXT_CHUNKS": "9",
            "SILICONFLOW_TIMEOUT_SECONDS": "30",
            "OLLAMA_TIMEOUT_SECONDS": "210",
        }
        with patch.dict(os.environ, env, clear=False):
            cfg = Config.load()
        self.assertEqual(cfg.api_key, "k_test")
        self.assertEqual(cfg.chat_model, "Pro/MiniMaxAI/MiniMax-M2.5")
        self.assertEqual(cfg.embedding_model, "Qwen/Qwen3-Embedding-4B")
        self.assertEqual(cfg.max_context_chunks, 9)
        self.assertIn("ollama", cfg.provider_chat_models)
        self.assertEqual(cfg.provider_chat_models.get("openai"), "gpt-4o")
        self.assertEqual(cfg.ollama_timeout_seconds, 210)

    def test_load_default_ollama_timeout(self) -> None:
        env = {
            "SILICONFLOW_TIMEOUT_SECONDS": "30",
            "OLLAMA_TIMEOUT_SECONDS": "",
            "OLLAMA_CHAT_MODEL": "",
        }
        with patch.dict(os.environ, env, clear=False):
            cfg = Config.load()
        self.assertEqual(cfg.ollama_timeout_seconds, 15)
        self.assertEqual(cfg.provider_chat_models.get("ollama"), "qwen3.5:latest")
        self.assertEqual(cfg.provider_chat_models.get("openai"), "gpt-4o-mini")


if __name__ == "__main__":
    unittest.main()
