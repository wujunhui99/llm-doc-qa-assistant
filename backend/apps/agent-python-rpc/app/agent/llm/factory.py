from __future__ import annotations

import os
from typing import Dict

from app.agent.llm.base import BaseChatClient
from app.agent.llm.claude_client import ClaudeClient
from app.agent.llm.ollama_client import OllamaClient
from app.agent.llm.openai_client import OpenAIClient
from app.agent.llm.siliconflow_client import SiliconFlowClient
from app.config import Config


def build_chat_clients(cfg: Config) -> Dict[str, BaseChatClient]:
    clients: Dict[str, BaseChatClient] = {
        "siliconflow": SiliconFlowClient(cfg.api_base, cfg.api_key, cfg.timeout_seconds),
    }

    # OpenAI and Ollama are active chat providers; Claude remains a reserved stub.
    clients["openai"] = OpenAIClient(
        api_base=(os.getenv("OPENAI_API_BASE", "https://api.openai.com/v1").strip() or "https://api.openai.com/v1"),
        api_key=os.getenv("OPENAI_API_KEY", "").strip(),
        timeout_seconds=cfg.timeout_seconds,
    )
    clients["claude"] = ClaudeClient(
        api_base=(os.getenv("ANTHROPIC_API_BASE", "https://api.anthropic.com/v1").strip() or "https://api.anthropic.com/v1"),
        api_key=os.getenv("ANTHROPIC_API_KEY", "").strip(),
        timeout_seconds=cfg.timeout_seconds,
    )
    clients["ollama"] = OllamaClient(
        api_base=(os.getenv("OLLAMA_API_BASE", "http://127.0.0.1:11434").strip() or "http://127.0.0.1:11434"),
        timeout_seconds=cfg.ollama_timeout_seconds,
    )
    return clients


# Backward-compatible alias for previous naming.
def build_provider_clients(cfg: Config) -> Dict[str, BaseChatClient]:
    return build_chat_clients(cfg)
