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

func TestLooksLikeCorruptedPDFText(t *testing.T) {
	bad := "嫒渺葖鉖澭䤓ὲ嘴馡刮´퀞牪⯹ﺅ쏤㪬䓁붎꼝眵㐮棙ݤ쇵깥ꍏ邈험魮砭塷㢻"
	if !LooksLikeCorruptedPDFText(bad) {
		t.Fatalf("expected mojibake-like text to be detected as corrupted")
	}
	good := "项目概述：设计并实现一个基于AI的智能文档问答系统，支持用户上传文档并进行多轮问答。"
	if LooksLikeCorruptedPDFText(good) {
		t.Fatalf("expected normal chinese text not to be detected as corrupted")
	}
}
