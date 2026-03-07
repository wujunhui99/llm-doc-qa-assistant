import unittest
from unittest.mock import patch

from app.agent.llm.claude_client import ClaudeClient


class ClaudeClientTestCase(unittest.TestCase):
    def test_chat_completion(self) -> None:
        client = ClaudeClient(
            api_base="https://api.anthropic.com/v1",
            api_key="k_test",
            timeout_seconds=20,
        )
        fake_resp = {"choices": [{"message": {"content": "你好，我是 Claude"}}]}
        with patch("app.agent.llm.claude_client.litellm_completion", return_value=fake_resp) as mocked:
            out = client.chat_completion([{"role": "user", "content": "你好"}], model="claude-3-5-haiku-latest", temperature=0.2)
        self.assertIn("Claude", out)
        payload = mocked.call_args.args[0]
        self.assertEqual(payload["model"], "anthropic/claude-3-5-haiku-latest")

    def test_chat_completion_with_tools(self) -> None:
        client = ClaudeClient(
            api_base="https://api.anthropic.com/v1",
            api_key="k_test",
            timeout_seconds=20,
        )
        fake_resp = {
            "choices": [
                {
                    "message": {
                        "content": "",
                        "tool_calls": [
                            {
                                "id": "call_1",
                                "function": {"name": "retrieval", "arguments": "{\"query\":\"llm\"}"},
                            }
                        ],
                    }
                }
            ]
        }
        with patch("app.agent.llm.claude_client.litellm_completion", return_value=fake_resp):
            out = client.chat_completion_with_tools(
                [{"role": "user", "content": "怎么实现"}],
                model="claude-3-5-haiku-latest",
                temperature=0.2,
                tools=[{"type": "function", "function": {"name": "retrieval"}}],
            )
        self.assertEqual(len(out.get("tool_calls") or []), 1)
        self.assertEqual((out.get("tool_calls") or [])[0].get("name"), "retrieval")


if __name__ == "__main__":
    unittest.main()
