import unittest
from unittest.mock import Mock, patch

import requests

from app.agent.llm.ollama_client import OllamaClient


class OllamaClientTestCase(unittest.TestCase):
    def test_chat_completion(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=15)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.return_value = {"message": {"content": "你好，我是 ollama"}}
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp) as mock_post:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2)
        self.assertIn("ollama", out)
        self.assertTrue(mock_post.called)
        kwargs = mock_post.call_args.kwargs
        self.assertEqual(kwargs["timeout"], 15)
        self.assertEqual(kwargs["json"]["think"], False)

    def test_chat_completion_sends_think_mode_flag(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=15)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.return_value = {"message": {"content": "ok"}}
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp) as mock_post:
            out = client.chat_completion(
                [{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2, think_mode=True
            )
        self.assertEqual(out, "ok")
        kwargs = mock_post.call_args.kwargs
        self.assertEqual(kwargs["json"]["think"], True)

    def test_chat_completion_strips_api_base_before_request(self) -> None:
        client = OllamaClient(api_base="  http://127.0.0.1:11434/  ", timeout_seconds=15)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.return_value = {"message": {"content": "ok"}}
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp) as mock_post:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2)
        self.assertEqual(out, "ok")
        self.assertTrue(mock_post.called)
        args, _ = mock_post.call_args
        self.assertEqual(args[0], "http://127.0.0.1:11434/api/chat")

    def test_chat_completion_allows_non_string_content(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=15)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.return_value = {"message": {"content": 12345}}
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp):
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2)
        self.assertEqual(out, "12345")

    def test_chat_completion_invalid_json_raises_runtime_error(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=15)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.side_effect = ValueError("invalid json")
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp):
            with self.assertRaises(RuntimeError) as cm:
                client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2)
        self.assertIn("invalid json", str(cm.exception).lower())

    def test_chat_completion_retries_once_on_read_timeout(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.return_value = {"message": {"content": "重试成功"}}
        with patch(
            "app.agent.llm.ollama_client.requests.post",
            side_effect=[requests.exceptions.ReadTimeout("timeout"), fake_resp],
        ) as mock_post:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2)
        self.assertEqual(out, "重试成功")
        self.assertEqual(mock_post.call_count, 2)
        first_call = mock_post.call_args_list[0]
        second_call = mock_post.call_args_list[1]
        self.assertEqual(first_call.kwargs["timeout"], 30)
        self.assertGreater(second_call.kwargs["timeout"], first_call.kwargs["timeout"])

    def test_chat_completion_stream_emits_delta_chunks(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.iter_lines.return_value = [
            '{"message":{"content":"你"}}',
            '{"message":{"content":"好"}}',
            '{"done":true}',
        ]
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp):
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2))
        self.assertEqual(
            chunks,
            [
                {"delta": "你", "thinking_delta": ""},
                {"delta": "好", "thinking_delta": ""},
            ],
        )

    def test_chat_completion_stream_emits_thinking_chunks(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.iter_lines.return_value = [
            '{"message":{"thinking":"先分析一下"}}',
            '{"message":{"content":"答案"}}',
            '{"done":true}',
        ]
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp):
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2))
        self.assertEqual(
            chunks,
            [
                {"delta": "", "thinking_delta": "先分析一下"},
                {"delta": "答案", "thinking_delta": ""},
            ],
        )

    def test_chat_completion_stream_sends_think_mode_flag(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.iter_lines.return_value = [
            '{"message":{"content":"ok"}}',
            '{"done":true}',
        ]
        with patch("app.agent.llm.ollama_client.requests.post", return_value=fake_resp) as mock_post:
            chunks = list(
                client.chat_completion_stream(
                    [{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2, think_mode=True
                )
            )
        self.assertEqual(chunks, [{"delta": "ok", "thinking_delta": ""}])
        kwargs = mock_post.call_args.kwargs
        self.assertEqual(kwargs["json"]["think"], True)

    def test_chat_completion_stream_retries_once_on_read_timeout(self) -> None:
        client = OllamaClient(api_base="http://127.0.0.1:11434", timeout_seconds=30)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.iter_lines.return_value = [
            '{"message":{"content":"ok"}}',
            '{"done":true}',
        ]
        with patch(
            "app.agent.llm.ollama_client.requests.post",
            side_effect=[requests.exceptions.ReadTimeout("timeout"), fake_resp],
        ) as mock_post:
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="qwen3:4b", temperature=0.2))
        self.assertEqual(chunks, [{"delta": "ok", "thinking_delta": ""}])
        self.assertEqual(mock_post.call_count, 2)
        first_call = mock_post.call_args_list[0]
        second_call = mock_post.call_args_list[1]
        self.assertEqual(first_call.kwargs["timeout"], 30)
        self.assertGreater(second_call.kwargs["timeout"], first_call.kwargs["timeout"])


if __name__ == "__main__":
    unittest.main()
