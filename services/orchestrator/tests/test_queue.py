import json
import os
from unittest.mock import AsyncMock

import pytest
import pytest_asyncio

from app.queue import redis as queue_module
from app.queue.redis import (
    QUEUE,
    close,
    ping,
    pop_job,
    push_job,
    queue_length,
)


@pytest.mark.asyncio
async def test_push_job_serializes_payload_to_json():
    rdb = AsyncMock()

    await push_job(rdb, {"jobId": "1", "documentId": "2"})

    rdb.lpush.assert_awaited_once_with(
        QUEUE, '{"jobId": "1", "documentId": "2"}'
    )


@pytest.mark.asyncio
async def test_pop_job_returns_payload_string():
    rdb = AsyncMock()
    rdb.brpop.return_value = (QUEUE, '{"jobId":"1","documentId":"2"}')

    raw = await pop_job(rdb, timeout=3)

    assert raw == '{"jobId":"1","documentId":"2"}'
    rdb.brpop.assert_awaited_once_with(QUEUE, timeout=3)


@pytest.mark.asyncio
async def test_pop_job_returns_none_on_timeout():
    rdb = AsyncMock()
    rdb.brpop.return_value = None

    assert await pop_job(rdb) is None


@pytest.mark.asyncio
async def test_queue_length_returns_number():
    rdb = AsyncMock()
    rdb.llen.return_value = 3

    assert await queue_length(rdb) == 3
    rdb.llen.assert_awaited_once_with(QUEUE)


@pytest.mark.asyncio
async def test_ping_is_awaited():
    rdb = AsyncMock()
    rdb.ping.return_value = True

    assert await ping(rdb) is True
    rdb.ping.assert_awaited_once()


@pytest.mark.asyncio
async def test_close_is_awaited():
    rdb = AsyncMock()

    await close(rdb)

    rdb.aclose.assert_awaited_once()


def test_get_redis_client_lets_server_control_blocking_timeout(monkeypatch):
    captured = {}

    def fake_from_url(url, **kwargs):
        captured["url"] = url
        captured["kwargs"] = kwargs
        return object()

    monkeypatch.setattr(queue_module.aioredis, "from_url", fake_from_url)
    monkeypatch.setattr(queue_module, "get_redis_url", lambda: "redis://test:6379")

    queue_module.get_redis_client()

    assert captured["url"] == "redis://test:6379"
    assert captured["kwargs"]["decode_responses"] is True
    assert captured["kwargs"]["socket_timeout"] is None


@pytest_asyncio.fixture
async def real_redis(monkeypatch):
    url = os.getenv("TEST_REDIS_URL", "redis://localhost:6379")
    monkeypatch.setattr(queue_module, "get_redis_url", lambda: url)
    # Never share the live queue with a running worker.
    monkeypatch.setattr(queue_module, "QUEUE", "jobs:test")

    rdb = queue_module.get_redis_client()
    try:
        await rdb.ping()
    except Exception as exc:  # pragma: no cover - depends on environment
        await rdb.aclose()
        pytest.skip(f"Redis is not available: {exc}")

    await rdb.delete("jobs:test")
    try:
        yield rdb
    finally:
        await rdb.delete("jobs:test")
        await rdb.aclose()


@pytest.mark.integration
@pytest.mark.asyncio
async def test_push_pop_roundtrip(real_redis):
    await push_job(real_redis, {"jobId": "1", "documentId": "2"})

    raw = await pop_job(real_redis, timeout=1)

    assert json.loads(raw) == {"jobId": "1", "documentId": "2"}


@pytest.mark.integration
@pytest.mark.asyncio
async def test_jobs_are_processed_in_fifo_order(real_redis):
    await push_job(real_redis, {"n": 1})
    await push_job(real_redis, {"n": 2})

    first = json.loads(await pop_job(real_redis, timeout=1))
    second = json.loads(await pop_job(real_redis, timeout=1))

    assert first["n"] == 1
    assert second["n"] == 2


@pytest.mark.integration
@pytest.mark.asyncio
async def test_queue_length_reflects_pushed_jobs(real_redis):
    assert await queue_length(real_redis) == 0

    await push_job(real_redis, {"n": 1})
    await push_job(real_redis, {"n": 2})

    assert await queue_length(real_redis) == 2
