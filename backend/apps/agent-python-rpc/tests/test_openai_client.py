import unittest
from unittest.mock import Mock, patch

from app.agent.llm.openai_client import OpenAIClient


class OpenAIClientTestCase(unittest.TestCase):
    def test_chat_completion(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.return_value = {
            "choices": [
                {"message": {"content": "你好，我是 OpenAI"}}
            ]
        }
        with patch("app.agent.llm.openai_client.requests.post", return_value=fake_resp) as mock_post:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2)
        self.assertIn("OpenAI", out)
        kwargs = mock_post.call_args.kwargs
        self.assertEqual(kwargs["timeout"], 20)
        self.assertEqual(kwargs["json"]["model"], "gpt-4o-mini")
        self.assertEqual(kwargs["headers"]["Authorization"], "Bearer k_test")

    def test_chat_completion_accepts_content_parts(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.json.return_value = {
            "choices": [
                {"message": {"content": [{"type": "text", "text": "你好"}, {"type": "text", "text": "世界"}]}}
            ]
        }
        with patch("app.agent.llm.openai_client.requests.post", return_value=fake_resp):
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2)
        self.assertEqual(out, "你好世界")

    def test_chat_completion_status_error(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = Mock()
        fake_resp.status_code = 401
        fake_resp.text = "unauthorized"
        with patch("app.agent.llm.openai_client.requests.post", return_value=fake_resp):
            with self.assertRaises(RuntimeError) as cm:
                client.chat_completion([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2)
        self.assertIn("status=401", str(cm.exception))

    def test_chat_completion_stream_emits_delta_chunks(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.iter_lines.return_value = [
            'data: {"choices":[{"delta":{"content":"你"}}]}',
            'data: {"choices":[{"delta":{"content":"好"}}]}',
            "data: [DONE]",
        ]
        with patch("app.agent.llm.openai_client.requests.post", return_value=fake_resp):
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2))
        self.assertEqual(chunks, [{"delta": "你", "thinking_delta": ""}, {"delta": "好", "thinking_delta": ""}])

    def test_chat_completion_stream_empty_raises(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = Mock()
        fake_resp.status_code = 200
        fake_resp.iter_lines.return_value = ["data: [DONE]"]
        with patch("app.agent.llm.openai_client.requests.post", return_value=fake_resp):
            with self.assertRaises(RuntimeError) as cm:
                list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2))
        self.assertIn("empty answer", str(cm.exception).lower())


if __name__ == "__main__":
    unittest.main()
