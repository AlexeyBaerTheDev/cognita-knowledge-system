import httpx
import pytest

from app.db.db import search_chunks
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
def search_calls(monkeypatch, fake_pool):
    calls = {"embed": [], "search": []}

    async def fake_embed(texts):
        calls["embed"].append(texts)
        return [[0.1, 0.2, 0.3]]

    async def fake_search_chunks(pool, vector, limit):
        calls["search"].append((pool, vector, limit))
        return [
            {
                "document_id": "doc-1",
                "chunk_index": 0,
                "content": "hello",
                "score": 0.9,
            }
        ]

    monkeypatch.setattr("app.main.embed", fake_embed)
    monkeypatch.setattr("app.main.search_chunks", fake_search_chunks)
    app.state.pool = fake_pool

    return calls


async def test_health():
    async with make_client() as client:
        response = await client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


async def test_search_ok(search_calls, fake_pool):
    async with make_client() as client:
        response = await client.post(
            "/search",
            json={"query": "  кошка  ", "limit": 3},
        )

    assert response.status_code == 200
    assert response.json() == {
        "results": [
            {
                "documentId": "doc-1",
                "chunkIndex": 0,
                "content": "hello",
                "score": 0.9,
            }
        ]
    }

    assert search_calls["embed"] == [["кошка"]]
    assert search_calls["search"] == [(fake_pool, [0.1, 0.2, 0.3], 3)]


async def test_search_empty_results(monkeypatch, fake_pool):
    async def fake_embed(texts):
        return [[0.1, 0.2, 0.3]]

    async def fake_search_chunks(pool, vector, limit):
        return []

    monkeypatch.setattr("app.main.embed", fake_embed)
    monkeypatch.setattr("app.main.search_chunks", fake_search_chunks)
    app.state.pool = fake_pool

    async with make_client() as client:
        response = await client.post("/search", json={"query": "nothing"})

    assert response.status_code == 200
    assert response.json() == {"results": []}


@pytest.mark.parametrize(
    "payload",
    [
        {"query": ""},
        {"query": "   "},
        {"query": "x", "limit": 0},
        {"query": "x", "limit": 21},
    ],
)
async def test_search_validation(search_calls, payload):
    async with make_client() as client:
        response = await client.post("/search", json=payload)

    assert response.status_code == 400
    assert search_calls["embed"] == []
    assert search_calls["search"] == []


async def test_search_missing_query(search_calls):
    async with make_client() as client:
        response = await client.post("/search", json={})

    assert response.status_code == 422
    assert search_calls["embed"] == []
    assert search_calls["search"] == []


class FakePool:
    def __init__(self, rows):
        self.rows = rows
        self.query = None
        self.args = None

    async def fetch(self, query, *args):
        self.query = query
        self.args = args
        return self.rows


async def test_search_chunks():
    rows = [
        {
            "document_id": "doc-1",
            "chunk_index": 2,
            "content": "chunk text",
            "score": 0.5,
        }
    ]
    pool = FakePool(rows)

    result = await search_chunks(pool, [0.1, 0.2], 3)

    assert result == rows
    assert "ORDER BY c.embedding <=> $1" in pool.query
    assert "LIMIT $2" in pool.query
    assert "d.status = 'READY'" in pool.query
    assert pool.args == ([0.1, 0.2], 3)


async def test_search_chunks_empty():
    pool = FakePool([])

    result = await search_chunks(pool, [0.1], 5)

    assert result == []
    assert pool.args == ([0.1], 5)
