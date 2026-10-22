import httpx
import pytest

from app.ollama import ollama as ollama_module
from app.ollama.ollama import embed


class FakeResponse:
    def __init__(self, payload, error=None):
        self._payload = payload
        self._error = error

    def raise_for_status(self):
        if self._error is not None:
            raise self._error

    def json(self):
        return self._payload


class FakeClient:
    calls: list[tuple[str, dict]] = []
    response_factory = None

    def __init__(self, *args, **kwargs):
        pass

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        return False

    async def post(self, url, json):
        FakeClient.calls.append((url, json))
        return FakeClient.response_factory(json)


@pytest.fixture
def config(monkeypatch):
    monkeypatch.setattr(ollama_module, "get_embedding_model", lambda: "embeddinggemma")
    monkeypatch.setattr(ollama_module, "get_ollama_url", lambda: "http://ollama:11434")


@pytest.fixture
def fake_http(monkeypatch):
    FakeClient.calls = []
    FakeClient.response_factory = lambda payload: FakeResponse({"embeddings": []})
    monkeypatch.setattr(ollama_module.httpx, "AsyncClient", FakeClient)
    return FakeClient


@pytest.mark.asyncio
async def test_embed_returns_vectors_and_sends_expected_request(config, fake_http):
    FakeClient.response_factory = lambda payload: FakeResponse(
        {"embeddings": [[0.1, 0.2], [0.3, 0.4]]}
    )

    result = await embed(["a", "b"])

    assert result == [[0.1, 0.2], [0.3, 0.4]]
    assert len(FakeClient.calls) == 1

    url, payload = FakeClient.calls[0]
    assert url == "http://ollama:11434/api/embed"
    assert payload == {"model": "embeddinggemma", "input": ["a", "b"]}


@pytest.mark.asyncio
async def test_embed_splits_into_batches_and_preserves_order(config, fake_http):
    FakeClient.response_factory = lambda payload: FakeResponse(
        {"embeddings": [[float(ord(t))] for t in payload["input"]]}
    )

    result = await embed(["a", "b", "c", "d", "e"], batch_size=2)

    assert [call[1]["input"] for call in FakeClient.calls] == [
        ["a", "b"],
        ["c", "d"],
        ["e"],
    ]
    assert result == [[97.0], [98.0], [99.0], [100.0], [101.0]]


@pytest.mark.asyncio
async def test_embed_empty_list_skips_http(fake_http):
    result = await embed([])

    assert result == []
    assert FakeClient.calls == []


@pytest.mark.asyncio
async def test_missing_model_raises(monkeypatch, fake_http):
    monkeypatch.setattr(ollama_module, "get_embedding_model", lambda: None)

    with pytest.raises(RuntimeError, match="embedding_model"):
        await embed(["a"])


@pytest.mark.asyncio
async def test_missing_url_raises(monkeypatch, fake_http):
    monkeypatch.setattr(ollama_module, "get_embedding_model", lambda: "m")
    monkeypatch.setattr(ollama_module, "get_ollama_url", lambda: None)

    with pytest.raises(ValueError, match="ollama_url"):
        await embed(["a"])


@pytest.mark.asyncio
async def test_http_error_propagates(config, fake_http):
    request = httpx.Request("POST", "http://ollama:11434/api/embed")
    response = httpx.Response(500, request=request)
    error = httpx.HTTPStatusError("500", request=request, response=response)

    FakeClient.response_factory = lambda payload: FakeResponse({}, error=error)

    with pytest.raises(httpx.HTTPStatusError):
        await embed(["a"])


@pytest.mark.asyncio
async def test_wrong_embedding_count_raises(config, fake_http):
    FakeClient.response_factory = lambda payload: FakeResponse(
        {"embeddings": [[0.1]]}
    )

    with pytest.raises(ValueError, match="returned 1 embeddings"):
        await embed(["a", "b"])


@pytest.mark.asyncio
async def test_unexpected_response_raises(config, fake_http):
    FakeClient.response_factory = lambda payload: FakeResponse({"foo": []})

    with pytest.raises(ValueError, match="unexpected ollama response"):
        await embed(["a"])


@pytest.mark.asyncio
async def test_invalid_batch_size_raises(config, fake_http):
    with pytest.raises(ValueError, match="batch_size"):
        await embed(["a"], batch_size=0)


@pytest.mark.integration
@pytest.mark.asyncio
async def test_embed_live(monkeypatch):
    monkeypatch.setenv("OLLAMA_URL", "http://localhost:11434")
    monkeypatch.setenv("EMBEDDING_MODEL", "embeddinggemma")

    vectors = await embed(["привет", "мир"])

    assert len(vectors) == 2
    assert all(len(v) == 768 for v in vectors)
