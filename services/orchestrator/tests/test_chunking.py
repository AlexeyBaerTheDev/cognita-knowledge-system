import pytest

from app.pipeline.chunking import chunk_text


def test_empty_text_returns_no_chunks():
    assert chunk_text("") == []
    assert chunk_text("   ") == []


def test_short_text_returns_single_chunk():
    chunks = chunk_text("hello")

    assert len(chunks) == 1
    assert chunks[0].text == "hello"
    assert (chunks[0].start, chunks[0].end) == (0, 5)
    assert chunks[0].index == 0


def test_long_text_is_split_with_expected_offsets():
    text = "a" * 2500

    chunks = chunk_text(text, size=1000, overlap=100)

    assert len(chunks) == 3
    assert [c.start for c in chunks] == [0, 900, 1800]
    assert [c.end for c in chunks] == [1000, 1900, 2500]
    assert [c.index for c in chunks] == [0, 1, 2]


def test_chunk_text_matches_original_slice():
    text = "Кошка сидела на подоконнике. " * 100

    chunks = chunk_text(text, size=500, overlap=50)

    assert chunks
    assert all(c.text == text[c.start:c.end] for c in chunks)


def test_adjacent_chunks_overlap():
    text = "b" * 2500

    chunks = chunk_text(text, size=1000, overlap=100)

    assert chunks[0].text[-100:] == chunks[1].text[:100]


def test_whitespace_only_window_is_skipped():
    text = "x" + " " * 3000 + "y"

    chunks = chunk_text(text, size=1000, overlap=100)

    assert all(c.text.strip() for c in chunks)
    assert [c.index for c in chunks] == list(range(len(chunks)))


@pytest.mark.parametrize(
    "size,overlap",
    [
        (0, 0),
        (-1, 0),
        (10, 10),
        (10, 20),
        (10, -1),
    ],
)
def test_invalid_window_parameters_raise(size, overlap):
    with pytest.raises(ValueError):
        chunk_text("some text", size=size, overlap=overlap)