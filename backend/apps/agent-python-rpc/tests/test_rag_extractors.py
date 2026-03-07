import unittest

from app.agent.rag.document_extractor import DocumentExtractionService
from app.agent.rag.markdown_extractor import MarkdownDocumentExtractor
from app.agent.rag.text_extractor import TextDocumentExtractor


class RagExtractorsTestCase(unittest.TestCase):
    def test_markdown_extractor_normalizes_markdown(self) -> None:
        ext = MarkdownDocumentExtractor()
        out = ext.extract(
            filename="spec.md",
            mime_type="text/markdown",
            content="# 项⽬概述\n- 系统支持`文档问答`\n- [说明](https://example.com)".encode("utf-8"),
        )
        self.assertEqual(out, "项目概述 系统支持文档问答 说明")

    def test_text_extractor_supports_plain_text(self) -> None:
        ext = TextDocumentExtractor()
        out = ext.extract(filename="a.txt", mime_type="text/plain", content="hello\nworld".encode("utf-8"))
        self.assertEqual(out, "hello world")

    def test_document_extraction_service_routes_by_type(self) -> None:
        svc = DocumentExtractionService()
        out = svc.extract_document_text(
            filename="readme.markdown",
            mime_type="text/markdown",
            content="## 标题\n正文".encode("utf-8"),
        )
        self.assertEqual(out, "标题 正文")

    def test_document_extraction_service_rejects_unknown_type(self) -> None:
        svc = DocumentExtractionService()
        with self.assertRaises(ValueError):
            svc.extract_document_text("a.exe", "application/octet-stream", b"abc")


if __name__ == "__main__":
    unittest.main()
