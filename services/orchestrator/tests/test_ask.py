import httpx
import pytest

import app.main as main_module
from app.main import app


def make_client() -> httpx.AsyncClient:
    transport = httpx.ASGITransport(app=app)
    return httpx.AsyncClient(transport=transport, base_url="http://test")


@pytest.fixture
def fake_pool():
    class Pool:
        pass

    return Pool()


@pytest.fixture
def ask_calls(monkeypatch, fake_pool):
    calls = {}

    async def fake_answer_question(pool, question, limit):
        calls["pool"] = pool
        calls["question"] = question
        calls["limit"] = limit
        return {
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

    monkeypatch.setattr(
        main_module,
        "answer_question",
        fake_answer_question,
        raising=False,
    )
    app.state.pool = fake_pool

    return calls


async def test_ask_ok(ask_calls, fake_pool):
    async with make_client() as client:
        response = await client.post(
            "/ask",
            json={"question": "  Где сидела кошка?  ", "limit": 3},
        )

    assert response.status_code == 200
    assert response.json() == {
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

    assert ask_calls["pool"] is fake_pool
    assert ask_calls["question"].strip() == "Где сидела кошка?"
    assert ask_calls["limit"] == 3


async def test_ask_default_limit(ask_calls):
    async with make_client() as client:
        response = await client.post(
            "/ask",
            json={"question": "Где сидела кошка?"},
        )

    assert response.status_code == 200
    assert ask_calls["limit"] == 5


async def test_ask_validation_error(monkeypatch, fake_pool):
    async def fake_answer_question(pool, question, limit):
        raise ValueError("question is required")

    monkeypatch.setattr(
        main_module,
        "answer_question",
        fake_answer_question,
        raising=False,
    )
    app.state.pool = fake_pool

    async with make_client() as client:
        response = await client.post("/ask", json={"question": "   "})

    assert response.status_code == 400
    assert response.json()["detail"] == "question is required"


async def test_ask_missing_question(ask_calls):
    async with make_client() as client:
        response = await client.post("/ask", json={})

    assert response.status_code == 422


async def test_ask_no_results(monkeypatch, fake_pool):
    async def fake_answer_question(pool, question, limit):
        return {
            "answer": "There is no information on this topic in the knowledge base.",
            "sources": [],
        }

    monkeypatch.setattr(
        main_module,
        "answer_question",
        fake_answer_question,
        raising=False,
    )
    app.state.pool = fake_pool

    async with make_client() as client:
        response = await client.post("/ask", json={"question": "кошка"})

    assert response.status_code == 200
    assert response.json() == {
        "answer": "There is no information on this topic in the knowledge base.",
        "sources": [],
    }
