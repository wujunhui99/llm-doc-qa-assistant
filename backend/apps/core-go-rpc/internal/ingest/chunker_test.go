package ingest

import "testing"

func TestChunkTextEmptyInput(t *testing.T) {
	got := ChunkText("   \n\t  ", 100, 20)
	if got != nil {
		t.Fatalf("expected nil for blank input, got %#v", got)
	}
}

func TestChunkTextKeepsWholeTextWhenShorterThanChunkSize(t *testing.T) {
	text := "  hello world  "
	got := ChunkText(text, 50, 10)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
	if got[0] != "hello world" {
		t.Fatalf("unexpected chunk: %q", got[0])
	}
}

func TestChunkTextOverlapWithUnicodeRunes(t *testing.T) {
	text := "天地玄黄宇宙洪荒日月盈昃"
	got := ChunkText(text, 4, 1) // step = 3

	want := []string{
		"天地玄黄",
		"黄宇宙洪",
		"洪荒日月",
		"月盈昃",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d chunks, got %d (%#v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chunk[%d] mismatch: want %q, got %q", i, want[i], got[i])
		}
	}
}

func TestChunkTextInvalidOverlapFallsBackToSafeStep(t *testing.T) {
	text := "abcdefghij"
	got := ChunkText(text, 3, 99) // invalid overlap -> fallback path

	// Current implementation falls back to overlap=120 then clamps step<=0 to chunkSize.
	// So this case behaves like non-overlap chunking with size=3.
	want := []string{"abc", "def", "ghi", "j"}
	if len(got) != len(want) {
		t.Fatalf("expected %d chunks, got %d (%#v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chunk[%d] mismatch: want %q, got %q", i, want[i], got[i])
		}
	}
}
