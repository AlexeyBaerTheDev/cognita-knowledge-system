import asyncio
import json
import logging
import signal

from app.config.config import get_database_url
from app.db.db import create_pool
from app.pipeline.ingest import process_job
from app.queue.redis import close, get_redis_client, pop_job

POLL_TIMEOUT = 5

logger = logging.getLogger(__name__)


def parse_payload(raw: str) -> tuple[str, str]:
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise ValueError("job payload must be a JSON object")

    job_id = data.get("jobId")
    document_id = data.get("documentId")
    if not job_id or not document_id:
        raise ValueError("job payload must contain jobId and documentId")

    return str(job_id), str(document_id)


async def handle_message(pool, raw: str) -> bool:
    try:
        job_id, document_id = parse_payload(raw)
    except (json.JSONDecodeError, ValueError):
        logger.exception("invalid job payload: %r", raw)
        return False

    try:
        await process_job(pool, job_id, document_id)
    except Exception:
        logger.exception("job %s crashed", job_id)
        return False

    return True


async def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    logging.getLogger("httpx").setLevel(logging.WARNING)

    pool = await create_pool(get_database_url())
    rdb = get_redis_client()
    stop = asyncio.Event()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, stop.set)
        except NotImplementedError:  # pragma: no cover - not on POSIX
            pass

    logger.info("worker started, waiting for jobs")
    try:
        while not stop.is_set():
            try:
                raw = await pop_job(rdb, timeout=POLL_TIMEOUT)
            except Exception:
                logger.exception("could not read from the queue")
                await asyncio.sleep(1)
                continue

            if raw is None:
                continue

            await handle_message(pool, raw)
    finally:
        await close(rdb)
        await pool.close()
        logger.info("worker stopped")


if __name__ == "__main__":
    asyncio.run(main())
