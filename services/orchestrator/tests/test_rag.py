import pytest

from app.rag import rag as rag_module
from app.rag.rag import NO_DATA_ANSWER, answer_question, build_messages


def test_build_messages_includes_question_and_numbered_sources():
    messages = build_messages(
        "Где сидела кошка?",
        [
            {"content": "Кошка сидела на подоконнике."},
            {"content": "Во дворе шёл дождь."},
        ],
    )

    assert messages[0]["role"] == "system"
    assert messages[1]["role"] == "user"

    system = messages[0]["content"]
    assert "knowledge base. Answer" in system
    assert "same language as the question" in system

    user = messages[1]["content"]
    assert "Где сидела кошка?" in user
    assert "[1] Кошка сидела на подоконнике." in user
    assert "[2] Во дворе шёл дождь." in user


def test_build_messages_rejects_empty_question():
    with pytest.raises(ValueError, match="question is required"):
        build_messages("   ", [{"content": "text"}])


def test_build_messages_rejects_empty_sources():
    with pytest.raises(ValueError, match="sources must not be empty"):
        build_messages("question", [])


def test_build_messages_rejects_empty_content():
    with pytest.raises(ValueError, match="empty content"):
        build_messages("question", [{"content": "   "}])


@pytest.mark.asyncio
async def test_answer_question_no_results_skips_chat(monkeypatch):
    async def fake_embed(texts):
        assert texts == ["кошка"]
        return [[0.1, 0.2]]

    async def fake_search(pool, vector, limit):
        assert vector == [0.1, 0.2]
        assert limit == 5
        return []

    chat_called = False

    async def fake_chat(messages):
        nonlocal chat_called
        chat_called = True
        return "should not be called"

    monkeypatch.setattr(rag_module, "embed", fake_embed)
    monkeypatch.setattr(rag_module, "search_chunks", fake_search)
    monkeypatch.setattr(rag_module, "chat", fake_chat)

    result = await answer_question(object(), "кошка", 5)

    assert result == {"answer": NO_DATA_ANSWER, "sources": []}
    assert chat_called is False


@pytest.mark.asyncio
async def test_answer_question_returns_answer_and_sources(monkeypatch):
    async def fake_embed(texts):
        assert texts == ["Где сидела кошка?"]
        return [[0.1, 0.2]]

    async def fake_search(pool, vector, limit):
        assert limit == 3
        return [
            {
                "document_id": "doc-1",
                "chunk_index": 0,
                "content": "Кошка сидела на подоконнике.",
                "score": 0.9,
            }
        ]

    async def fake_chat(messages):
        assert messages[0]["role"] == "system"
        assert "Где сидела кошка?" in messages[1]["content"]
        return "Кошка сидела на подоконнике [1]."

    monkeypatch.setattr(rag_module, "embed", fake_embed)
    monkeypatch.setattr(rag_module, "search_chunks", fake_search)
    monkeypatch.setattr(rag_module, "chat", fake_chat)

    result = await answer_question(object(), "Где сидела кошка?", 3)

    assert result == {
        "answer": "Кошка сидела на подоконнике [1].",
        "sources": [
            {
                "documentId": "doc-1",
                "chunkIndex": 0,
                "content": "Кошка сидела на подоконнике.",
                "score": 0.9,
            }
        ],
    }


@pytest.mark.asyncio
async def test_answer_question_rejects_empty_question(monkeypatch):
    async def fail_embed(texts):
        raise AssertionError("embed should not be called")

    monkeypatch.setattr(rag_module, "embed", fail_embed)

    with pytest.raises(ValueError, match="question is required"):
        await answer_question(object(), "   ", 5)


@pytest.mark.asyncio
@pytest.mark.parametrize("limit", [0, 21])
async def test_answer_question_rejects_invalid_limit(monkeypatch, limit):
    async def fail_embed(texts):
        raise AssertionError("embed should not be called")

    monkeypatch.setattr(rag_module, "embed", fail_embed)

    with pytest.raises(ValueError, match="limit must be between 1 and 20"):
        await answer_question(object(), "кошка", limit)


@pytest.mark.asyncio
async def test_answer_question_rejects_empty_embedding(monkeypatch):
    async def fake_embed(texts):
        return []

    monkeypatch.setattr(rag_module, "embed", fake_embed)

    with pytest.raises(RuntimeError, match="embedding returned no vectors"):
        await answer_question(object(), "кошка", 5)
