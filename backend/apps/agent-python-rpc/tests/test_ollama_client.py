import unittest
from unittest.mock import patch

from app.agent.llm.ollama_client import OllamaClient


class OllamaClientTestCase(unittest.TestCase):
    def test_chat_completion(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=15)
        fake_resp = {"choices": [{"message": {"content": "你好，我是 ollama"}}]}
        with patch("app.agent.llm.ollama_client.litellm_completion", return_value=fake_resp) as mocked:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2)
        self.assertIn("ollama", out)
        payload = mocked.call_args.args[0]
        self.assertEqual(payload["model"], "ollama/qwen3.5:latest")
        self.assertEqual(payload["timeout"], 15)
        self.assertEqual(payload["think"], False)
        self.assertEqual(payload["num_predict"], 2048)

    def test_chat_completion_sends_think_mode_flag(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=15)
        fake_resp = {"choices": [{"message": {"content": "ok"}}]}
        with patch("app.agent.llm.ollama_client.litellm_completion", return_value=fake_resp) as mocked:
            out = client.chat_completion(
                [{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2, think_mode=True
            )
        self.assertEqual(out, "ok")
        payload = mocked.call_args.args[0]
        self.assertEqual(payload["think"], True)

    def test_chat_completion_strips_api_base_before_request(self) -> None:
        client = OllamaClient(api_base="  http://127.0.0.1:11434/  ", timeout_seconds=15)
        fake_resp = {"choices": [{"message": {"content": "ok"}}]}
        with patch("app.agent.llm.ollama_client.litellm_completion", return_value=fake_resp) as mocked:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2)
        self.assertEqual(out, "ok")
        payload = mocked.call_args.args[0]
        self.assertEqual(payload["api_base"], "http://127.0.0.1:11434")

    def test_chat_completion_retries_once_on_timeout(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_resp = {"choices": [{"message": {"content": "重试成功"}}]}
        with patch(
            "app.agent.llm.ollama_client.litellm_completion",
            side_effect=[Exception("Read timed out"), fake_resp],
        ) as mocked:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2)
        self.assertEqual(out, "重试成功")
        self.assertEqual(mocked.call_count, 2)
        first_payload = mocked.call_args_list[0].args[0]
        second_payload = mocked.call_args_list[1].args[0]
        self.assertEqual(first_payload["timeout"], 30)
        self.assertGreater(second_payload["timeout"], first_payload["timeout"])

    def test_chat_completion_stream_emits_delta_chunks(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_stream = iter(
            [
                {"choices": [{"delta": {"content": "你"}}]},
                {"choices": [{"delta": {"content": "好"}}]},
            ]
        )
        with patch("app.agent.llm.ollama_client.litellm_completion", return_value=fake_stream):
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2))
        self.assertEqual(chunks, [{"delta": "你", "thinking_delta": ""}, {"delta": "好", "thinking_delta": ""}])

    def test_chat_completion_stream_ignores_thinking_chunks_when_think_mode_off(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_stream = iter(
            [
                {"choices": [{"delta": {"thinking": "先分析一下"}}]},
                {"choices": [{"delta": {"content": "答案"}}]},
            ]
        )
        with patch("app.agent.llm.ollama_client.litellm_completion", return_value=fake_stream):
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2))
        self.assertEqual(chunks, [{"delta": "答案", "thinking_delta": ""}])

    def test_chat_completion_stream_sends_think_mode_flag(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_stream = iter([{"choices": [{"delta": {"content": "ok"}}]}])
        with patch("app.agent.llm.ollama_client.litellm_completion", return_value=fake_stream) as mocked:
            chunks = list(
                client.chat_completion_stream(
                    [{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2, think_mode=True
                )
            )
        self.assertEqual(chunks, [{"delta": "ok", "thinking_delta": ""}])
        payload = mocked.call_args.args[0]
        self.assertEqual(payload["think"], True)

    def test_chat_completion_stream_retries_once_on_timeout(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_stream = iter([{"choices": [{"delta": {"content": "ok"}}]}])
        with patch(
            "app.agent.llm.ollama_client.litellm_completion",
            side_effect=[Exception("Read timed out"), fake_stream],
        ) as mocked:
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="qwen3.5:latest", temperature=0.2))
        self.assertEqual(chunks, [{"delta": "ok", "thinking_delta": ""}])
        self.assertEqual(mocked.call_count, 2)
        first_payload = mocked.call_args_list[0].args[0]
        second_payload = mocked.call_args_list[1].args[0]
        self.assertEqual(first_payload["timeout"], 30)
        self.assertGreater(second_payload["timeout"], first_payload["timeout"])


if __name__ == "__main__":
    unittest.main()
