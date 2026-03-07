import unittest

from app.agent.tools.retrieval_tool import parse_router_output


class RetrievalToolTestCase(unittest.TestCase):
    def test_parse_router_output_from_tool_calls(self) -> None:
        out = parse_router_output(
            content="",
            tool_calls=[
                {
                    "name": "retrieval",
                    "arguments": '{"query":"react rag architecture","reason":"需要文档证据"}',
                }
            ],
        )
        self.assertTrue(out["use_retrieval"])
        self.assertEqual(out["retrieval_query"], "react rag architecture")
        self.assertEqual(out["tool"], "retrieval")

    def test_parse_router_output_from_json_text(self) -> None:
        out = parse_router_output(
            content='{"use_retrieval":false,"reason":"small talk","retrieval_query":""}',
            tool_calls=[],
        )
        self.assertFalse(out["use_retrieval"])
        self.assertEqual(out["reason"], "small talk")


if __name__ == "__main__":
    unittest.main()
