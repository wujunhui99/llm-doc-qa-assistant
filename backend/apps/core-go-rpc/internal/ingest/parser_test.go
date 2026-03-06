package ingest

import "testing"

func TestParseDocumentTextNormalizesCompatibilityHan(t *testing.T) {
	raw := []byte("项⽬概述\n系统支持文档问答。")
	out, err := ParseDocumentText("a.md", "text/markdown", raw)
	if err != nil {
		t.Fatalf("parse markdown failed: %v", err)
	}
	if out != "项目概述 系统支持文档问答。" {
		t.Fatalf("unexpected normalized text: %q", out)
	}
}

func TestIsReadableText(t *testing.T) {
	if !IsReadableText("项目概述：系统支持上传、检索和问答。") {
		t.Fatalf("expected readable chinese text")
	}
	if IsReadableText("ὲ´⯹ﺅ쏤㪬퀞ݤ쇵깥ꍏ험魮Ⴭ") {
		t.Fatalf("expected unreadable gibberish text")
	}
}
