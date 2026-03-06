import os
import unittest

from app.config import Config


class ConfigTestCase(unittest.TestCase):
    def test_load_prefers_env_values(self) -> None:
        os.environ["SILICONFLOW_API_KEY"] = "k_test"
        os.environ["SILICONFLOW_CHAT_MODEL"] = "Pro/MiniMaxAI/MiniMax-M2.5"
        os.environ["SILICONFLOW_EMBEDDING_MODEL"] = "Qwen/Qwen3-Embedding-4B"
        os.environ["LLM_AGENT_MAX_CONTEXT_CHUNKS"] = "9"

        cfg = Config.load()
        self.assertEqual(cfg.api_key, "k_test")
        self.assertEqual(cfg.chat_model, "Pro/MiniMaxAI/MiniMax-M2.5")
        self.assertEqual(cfg.embedding_model, "Qwen/Qwen3-Embedding-4B")
        self.assertEqual(cfg.max_context_chunks, 9)


if __name__ == "__main__":
    unittest.main()
