import unittest
from unittest.mock import patch

from app.agent.llm.openai_client import OpenAIClient


class OpenAIClientTestCase(unittest.TestCase):
    def test_chat_completion(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = {
            "choices": [
                {"message": {"content": "你好，我是 OpenAI"}},
            ]
        }
        with patch("app.agent.llm.openai_client.litellm_completion", return_value=fake_resp) as mocked:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2)
        self.assertIn("OpenAI", out)
        payload = mocked.call_args.args[0]
        self.assertEqual(payload["model"], "openai/gpt-4o-mini")
        self.assertEqual(payload["timeout"], 20)
        self.assertEqual(payload["api_key"], "k_test")

    def test_chat_completion_accepts_content_parts(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = {
            "choices": [
                {"message": {"content": [{"type": "text", "text": "你好"}, {"type": "text", "text": "世界"}]}},
            ]
        }
        with patch("app.agent.llm.openai_client.litellm_completion", return_value=fake_resp):
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2)
        self.assertEqual(out, "你好世界")

    def test_chat_completion_request_error(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        with patch("app.agent.llm.openai_client.litellm_completion", side_effect=Exception("status=401 unauthorized")):
            with self.assertRaises(RuntimeError) as cm:
                client.chat_completion([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2)
        self.assertIn("request failed", str(cm.exception).lower())

    def test_chat_completion_stream_emits_delta_chunks(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_stream = iter(
            [
                {"choices": [{"delta": {"content": "你"}}]},
                {"choices": [{"delta": {"content": "好"}}]},
            ]
        )
        with patch("app.agent.llm.openai_client.litellm_completion", return_value=fake_stream):
            chunks = list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2))
        self.assertEqual(chunks, [{"delta": "你", "thinking_delta": ""}, {"delta": "好", "thinking_delta": ""}])

    def test_chat_completion_stream_empty_raises(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        with patch("app.agent.llm.openai_client.litellm_completion", return_value=iter([])):
            with self.assertRaises(RuntimeError) as cm:
                list(client.chat_completion_stream([{"role": "user", "content": "你好"}], model="gpt-4o-mini", temperature=0.2))
        self.assertIn("empty answer", str(cm.exception).lower())

    def test_chat_completion_with_tools(self) -> None:
        client = OpenAIClient(api_base="https://api.openai.com/v1", api_key="k_test", timeout_seconds=20)
        fake_resp = {
            "choices": [
                {
                    "message": {
                        "content": "",
                        "tool_calls": [
                            {
                                "id": "call_1",
                                "function": {
                                    "name": "retrieval",
                                    "arguments": "{\"query\":\"python react architecture\",\"reason\":\"技术选型\"}",
                                },
                            }
                        ],
                    }
                }
            ]
        }
        with patch("app.agent.llm.openai_client.litellm_completion", return_value=fake_resp):
            out = client.chat_completion_with_tools(
                [{"role": "user", "content": "怎么实现"}],
                model="gpt-4o-mini",
                temperature=0.2,
                tools=[{"type": "function", "function": {"name": "retrieval"}}],
                tool_choice="auto",
            )
        self.assertTrue(isinstance(out, dict))
        self.assertEqual(out.get("content"), "")
        calls = out.get("tool_calls") or []
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0].get("name"), "retrieval")


if __name__ == "__main__":
    unittest.main()
