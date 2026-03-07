from __future__ import annotations

import unicodedata


def normalize_text(text: str) -> str:
    if not text:
        return ""
    normalized = unicodedata.normalize("NFKC", text)
    normalized = normalized.replace("\u00a0", " ").replace("\u200b", "")
    normalized = normalized.replace("\r", "\n")
    lines = [line.strip() for line in normalized.split("\n")]
    return " ".join(" ".join(lines).split())


def _readable_ratio(text: str) -> float:
    if not text:
        return 0.0
    total = 0
    readable = 0
    for ch in text:
        total += 1
        code = ord(ch)
        if ch.isspace():
            readable += 1
            continue
        if 0x4E00 <= code <= 0x9FFF:
            readable += 1
            continue
        if ("a" <= ch <= "z") or ("A" <= ch <= "Z") or ("0" <= ch <= "9"):
            readable += 1
            continue
        if ch in "，。！？；：,.!?;:()[]{}<>-_/#'\"%+*":
            readable += 1
            continue
    if total == 0:
        return 0.0
    return readable / float(total)


def is_readable_text(text: str) -> bool:
    text = (text or "").strip()
    if len(text) < 16:
        return False
    return _readable_ratio(text) >= 0.45
