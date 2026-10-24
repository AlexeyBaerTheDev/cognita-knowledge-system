import json

import redis.asyncio as aioredis
from redis.asyncio import Redis

from app.config.config import get_redis_url

QUEUE = "jobs"
PROCESSING = "jobs:processing"


def get_redis_client() -> Redis:
    return aioredis.from_url(
        get_redis_url(),
        decode_responses=True,
        health_check_interval=30,
        socket_timeout=None,
    )


async def ping(rdb: Redis) -> bool:
    return await rdb.ping()


async def queue_length(rdb: Redis) -> int:
    return await rdb.llen(QUEUE)


async def push_job(rdb: Redis, payload: dict) -> None:
    await rdb.lpush(QUEUE, json.dumps(payload))


async def pop_job(rdb: Redis, timeout: int = 5) -> str | None:
    item = await rdb.brpop(QUEUE, timeout=timeout)
    return item[1] if item else None


async def close(rdb: Redis) -> None:
    await rdb.aclose()
