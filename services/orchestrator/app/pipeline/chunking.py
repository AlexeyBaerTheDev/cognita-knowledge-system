from dataclasses import dataclass

@dataclass
class Chunk:
    index: int
    text: str
    start: int
    end: int

def chunk_text(text: str, size: int = 1000, overlap: int = 150) -> list[Chunk]:
    if not text or not text.strip():
        return []
    if overlap >= size:
        raise ValueError("overlap must be smaller than size")
    if overlap < 0:
        raise ValueError("overlap must be non-negative")
    if size <= 0:
        raise ValueError("size must be positive")

    step = size - overlap
    chunks: list[Chunk] = []

    for start in range(0, len(text), step):
        end = min(start + size, len(text))
        raw = text[start:end]

        left = len(raw) - len(raw.lstrip())
        right = len(raw.rstrip())
        content = raw[left:right]

        if content:
            chunks.append(
                Chunk(
                    index = len(chunks),
                    text = content,
                    start = start + left,
                    end = start + right,
                )
            )

        if end == len(text):
            break

    return chunks
        