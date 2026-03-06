from __future__ import annotations

import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from app.config import load_config


class ConfigLoadTests(unittest.TestCase):
    def test_load_config_from_file_and_env_override(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            config_path = Path(tmp) / "cfg.json"
            config_path.write_text(
                json.dumps(
                    {
                        "provider": "siliconflow",
                        "siliconflow": {
                            "api_base": "https://api.siliconflow.cn/v1",
                            "chat_model": "Pro/MiniMaxAI/MiniMax-M2.5",
                            "embedding_model": "Qwen/Qwen3-Embedding-4B",
                            "timeout_seconds": 20,
                            "temperature": 0.1,
                        },
                        "agent": {
                            "max_context_chunks": 4,
                            "max_steps": 2,
                            "plan_temperature": 0,
                            "answer_temperature": 0.2,
                        },
                    }
                ),
                encoding="utf-8",
            )

            env = {
                "LLM_CONFIG_FILE": str(config_path),
                "SILICONFLOW_CHAT_MODEL": "Pro/MiniMaxAI/MiniMax-M2.5",
                "SILICONFLOW_EMBEDDING_MODEL": "Qwen/Qwen3-Embedding-4B",
                "SILICONFLOW_API_KEY": "abc",
            }
            with mock.patch.dict(os.environ, env, clear=True):
                cfg = load_config()

            self.assertEqual(cfg.siliconflow.api_key, "abc")
            self.assertEqual(cfg.siliconflow.chat_model, "Pro/MiniMaxAI/MiniMax-M2.5")
            self.assertEqual(cfg.siliconflow.embedding_model, "Qwen/Qwen3-Embedding-4B")
            self.assertEqual(cfg.agent.max_context_chunks, 4)


if __name__ == "__main__":
    unittest.main()
