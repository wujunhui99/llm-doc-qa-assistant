import json
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Dict


def load_dotenv(path: Path) -> None:
    if not path.exists():
        return
    for idx, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        raw = line.strip()
        if not raw or raw.startswith("#"):
            continue
        if raw.startswith("export "):
            raw = raw[len("export ") :].strip()
        if "=" not in raw:
            continue
        key, val = raw.split("=", 1)
        key = key.strip()
        val = val.strip()
        if len(val) >= 2 and ((val.startswith('"') and val.endswith('"')) or (val.startswith("'") and val.endswith("'"))):
            val = val[1:-1]
        if key and key not in os.environ:
            os.environ[key] = val


@dataclass(frozen=True)
class Config:
    host: str
    port: int
    listen_addr: str
    timeout_seconds: int
    api_base: str
    api_key: str
    chat_model: str
    embedding_model: str
    temperature: float
    default_provider: str
    provider_chat_models: Dict[str, str]
    max_context_chunks: int


    @classmethod
    def load(cls) -> "Config":
        service_root = Path(__file__).resolve().parent.parent
        load_dotenv(service_root.parent / "core-go-rpc" / ".env")
        load_dotenv(service_root / ".env")

        default_model = os.getenv("SILICONFLOW_CHAT_MODEL", "Pro/MiniMaxAI/MiniMax-M2.5").strip() or "Pro/MiniMaxAI/MiniMax-M2.5"
        provider_models = {
            "siliconflow": default_model,
            "mock": default_model,
            "openai": default_model,
            "claude": default_model,
            "local": default_model,
        }

        raw = os.getenv("SILICONFLOW_PROVIDER_CHAT_MODELS_JSON", "").strip()
        if raw:
            try:
                parsed = json.loads(raw)
                if isinstance(parsed, dict):
                    for k, v in parsed.items():
                        key = str(k).strip().lower()
                        val = str(v).strip()
                        if key and val:
                            provider_models[key] = val
            except Exception:
                pass

        timeout = int(os.getenv("SILICONFLOW_TIMEOUT_SECONDS", "30") or "30")
        if timeout <= 0:
            timeout = 30

        temp = float(os.getenv("SILICONFLOW_TEMPERATURE", "0.2") or "0.2")
        max_context_chunks = int(os.getenv("LLM_AGENT_MAX_CONTEXT_CHUNKS", "6") or "6")
        if max_context_chunks <= 0:
            max_context_chunks = 6

        host = os.getenv("LLM_RPC_HOST", "127.0.0.1").strip() or "127.0.0.1"
        port = int(os.getenv("LLM_RPC_PORT", "51000") or "51000")
        listen_addr = os.getenv("LLM_RPC_LISTEN_ADDR", "").strip()
        if not listen_addr:
            listen_addr = f"{host}:{port}"

        return cls(
            host=host,
            port=port,
            listen_addr=listen_addr,
            timeout_seconds=timeout,
            api_base=os.getenv("SILICONFLOW_API_BASE", "https://api.siliconflow.cn/v1").strip() or "https://api.siliconflow.cn/v1",
            api_key=os.getenv("SILICONFLOW_API_KEY", "").strip(),
            chat_model=default_model,
            embedding_model=os.getenv("SILICONFLOW_EMBEDDING_MODEL", "Qwen/Qwen3-Embedding-4B").strip() or "Qwen/Qwen3-Embedding-4B",
            temperature=temp,
            default_provider=os.getenv("LLM_PROVIDER", "siliconflow").strip().lower() or "siliconflow",
            provider_chat_models=provider_models,
            max_context_chunks=max_context_chunks,
        )
