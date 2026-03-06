from __future__ import annotations

import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class SiliconFlowConfig:
    api_base: str
    api_key: str
    chat_model: str
    embedding_model: str
    timeout_seconds: float
    temperature: float
    provider_chat_models: dict[str, str] = field(default_factory=dict)


@dataclass
class AgentConfig:
    max_context_chunks: int
    max_steps: int
    plan_temperature: float
    answer_temperature: float


@dataclass
class AppConfig:
    provider: str
    siliconflow: SiliconFlowConfig
    agent: AgentConfig


def load_config() -> AppConfig:
    service_root = Path(__file__).resolve().parent.parent
    default_config_path = service_root / "config" / "defaults.json"
    config_path = Path(os.getenv("LLM_CONFIG_FILE", str(default_config_path))).expanduser()

    raw = _load_json_config(config_path)
    _load_env_file(service_root / ".env")

    provider = _env_str("LLM_PROVIDER", raw.get("provider", "siliconflow"))

    sf_raw = raw.get("siliconflow", {}) if isinstance(raw.get("siliconflow"), dict) else {}
    chat_model = _env_str("SILICONFLOW_CHAT_MODEL", sf_raw.get("chat_model", "Pro/MiniMaxAI/MiniMax-M2.5"))
    provider_models = _parse_provider_model_mapping(os.getenv("SILICONFLOW_PROVIDER_CHAT_MODELS_JSON", ""), chat_model)

    siliconflow = SiliconFlowConfig(
        api_base=_env_str("SILICONFLOW_API_BASE", sf_raw.get("api_base", "https://api.siliconflow.cn/v1")),
        api_key=_env_str("SILICONFLOW_API_KEY", sf_raw.get("api_key", "")),
        chat_model=chat_model,
        embedding_model=_env_str("SILICONFLOW_EMBEDDING_MODEL", sf_raw.get("embedding_model", "Qwen/Qwen3-Embedding-4B")),
        timeout_seconds=_env_float("SILICONFLOW_TIMEOUT_SECONDS", sf_raw.get("timeout_seconds", 30.0)),
        temperature=_env_float("SILICONFLOW_TEMPERATURE", sf_raw.get("temperature", 0.2)),
        provider_chat_models=provider_models,
    )

    agent_raw = raw.get("agent", {}) if isinstance(raw.get("agent"), dict) else {}
    agent = AgentConfig(
        max_context_chunks=max(1, _env_int("LLM_AGENT_MAX_CONTEXT_CHUNKS", agent_raw.get("max_context_chunks", 6))),
        max_steps=max(1, _env_int("LLM_AGENT_MAX_STEPS", agent_raw.get("max_steps", 2))),
        plan_temperature=max(0.0, _env_float("LLM_AGENT_PLAN_TEMPERATURE", agent_raw.get("plan_temperature", 0.0))),
        answer_temperature=max(0.0, _env_float("LLM_AGENT_ANSWER_TEMPERATURE", agent_raw.get("answer_temperature", 0.2))),
    )

    return AppConfig(provider=provider, siliconflow=siliconflow, agent=agent)


def _load_json_config(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return {}


def _load_env_file(path: Path) -> None:
    if not path.exists():
        return
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        if key and key not in os.environ:
            os.environ[key] = value


def _env_str(key: str, default: Any) -> str:
    value = os.getenv(key)
    if value is None:
        return str(default).strip()
    return value.strip()


def _env_int(key: str, default: Any) -> int:
    value = os.getenv(key)
    if value is None:
        try:
            return int(default)
        except Exception:
            return 0
    try:
        return int(value.strip())
    except Exception:
        try:
            return int(default)
        except Exception:
            return 0


def _env_float(key: str, default: Any) -> float:
    value = os.getenv(key)
    if value is None:
        try:
            return float(default)
        except Exception:
            return 0.0
    try:
        return float(value.strip())
    except Exception:
        try:
            return float(default)
        except Exception:
            return 0.0


def _parse_provider_model_mapping(raw_json: str, default_model: str) -> dict[str, str]:
    mapping: dict[str, str] = {
        "siliconflow": default_model,
        "mock": default_model,
        "openai": default_model,
        "claude": default_model,
        "local": default_model,
    }
    raw_json = (raw_json or "").strip()
    if not raw_json:
        return mapping

    try:
        parsed = json.loads(raw_json)
    except Exception:
        return mapping

    if not isinstance(parsed, dict):
        return mapping

    for key, value in parsed.items():
        if not isinstance(key, str) or not isinstance(value, str):
            continue
        k = key.strip().lower()
        v = value.strip()
        if k and v:
            mapping[k] = v
    return mapping
