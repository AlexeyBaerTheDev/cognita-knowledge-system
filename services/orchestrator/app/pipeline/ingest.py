import logging

import asyncpg

from app.config.config import get_embedding_model
from app.db.db import (
    fetch_document,
    fetch_job,
    replace_chunks,
    set_document_status,
    set_job_done,
    set_job_failed,
    set_job_progress,
    set_job_running,
)
from app.ollama.ollama import embed
from app.pipeline.chunking import chunk_text

logger = logging.getLogger(__name__)


async def process_job(
    pool: asyncpg.Pool,
    job_id: str,
    document_id: str,
) -> None:
    job = await fetch_job(pool, job_id)
    if job is None:
        raise ValueError(f"job {job_id} not found")

    if job["status"] == "SUCCEEDED":
        logger.info("job %s is already SUCCEEDED, skipping", job_id)
        return

    job_document_id = job["document_id"]
    if job_document_id is not None and str(job_document_id) != document_id:
        raise ValueError(
            f"job {job_id} belongs to document {job_document_id}, "
            f"not {document_id}"
        )

    try:
        await set_job_running(pool, job_id)
        await set_document_status(pool, document_id, "PROCESSING")

        document = await fetch_document(pool, document_id)
        if document is None:
            raise ValueError(f"document {document_id} not found")

        text = document["content"]
        if not text or not text.strip():
            raise ValueError(f"document {document_id} has no content")

        chunks = chunk_text(text)
        if not chunks:
            raise ValueError(f"document {document_id} produced no chunks")
        await set_job_progress(pool, job_id, "chunk", 30)

        vectors = await embed([chunk.text for chunk in chunks])
        if len(vectors) != len(chunks):
            raise ValueError(
                f"embedding count mismatch: {len(vectors)} vectors "
                f"for {len(chunks)} chunks"
            )
        await set_job_progress(pool, job_id, "embed", 70)

        await replace_chunks(
            pool,
            document_id,
            chunks,
            vectors,
            embedding_model=get_embedding_model(),
        )
        await set_job_progress(pool, job_id, "store", 90)

        await set_document_status(pool, document_id, "READY")
        await set_job_done(pool, job_id)
        logger.info(
            "job %s done: document %s, %d chunks",
            job_id,
            document_id,
            len(chunks),
        )
    except Exception as err:
        await mark_failed(pool, job_id, document_id, err)
        raise


async def mark_failed(
    pool: asyncpg.Pool,
    job_id: str,
    document_id: str,
    err: Exception,
) -> None:
    try:
        await set_document_status(pool, document_id, "FAILED")
    except Exception:
        logger.exception("could not mark document %s as FAILED", document_id)

    try:
        await set_job_failed(pool, job_id, err)
    except Exception:
        logger.exception("could not mark job %s as FAILED", job_id)
