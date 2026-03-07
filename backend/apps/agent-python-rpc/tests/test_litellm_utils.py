import unittest

from app.agent.llm.litellm_utils import build_model_name


class LiteLLMUtilsTestCase(unittest.TestCase):
    def test_build_model_name_adds_prefix_for_unqualified_model(self) -> None:
        out = build_model_name("openai", "gpt-4o-mini")
        self.assertEqual(out, "openai/gpt-4o-mini")

    def test_build_model_name_adds_prefix_for_slash_model_without_provider(self) -> None:
        out = build_model_name("openai", "Pro/MiniMaxAI/MiniMax-M2.5")
        self.assertEqual(out, "openai/Pro/MiniMaxAI/MiniMax-M2.5")

    def test_build_model_name_keeps_same_provider_model(self) -> None:
        out = build_model_name("openai", "openai/gpt-4o-mini")
        self.assertEqual(out, "openai/gpt-4o-mini")

    def test_build_model_name_keeps_explicit_other_provider_model(self) -> None:
        out = build_model_name("openai", "azure/gpt-4o")
        self.assertEqual(out, "azure/gpt-4o")


if __name__ == "__main__":
    unittest.main()
