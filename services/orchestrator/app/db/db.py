import asyncpg
from pgvector.asyncpg import register_vector

async def create_pool(dsn: str) -> asyncpg.Pool:
    async def init(conn):
        await register_vector(conn)
    return await asyncpg.create_pool(dsn, init=init, min_size=1, max_size=10)

async def set_document_status(pool: asyncpg.Pool, doc_id: str, status: str) -> None:
    await pool.execute(
        """UPDATE documents SET status=$2, updated_at=now() WHERE id=$1::uuid""",
       doc_id, status,
    )

async def set_job_running(pool: asyncpg.Pool, job_id: str) -> None:
    await pool.execute(
        """UPDATE processing_jobs SET status='RUNNING', current_step='started', progress=0, error_message=NULL, started_at=now(), finished_at=NULL WHERE id=$1::uuid""",
        job_id
    )

async def set_job_progress(pool: asyncpg.Pool, job_id: str, step: str, progress: int) -> None:
    await pool.execute(
        "UPDATE processing_jobs SET current_step=$2, progress=$3 WHERE id=$1::uuid",
        job_id, step, progress,
    )

async def set_job_done(pool: asyncpg.Pool, job_id: str) -> None:
    await pool.execute(
        """UPDATE processing_jobs SET status='SUCCEEDED', current_step='done', progress=100, error_message=NULL, finished_at=now() WHERE id=$1::uuid""",
        job_id,
    )

async def set_job_failed(pool: asyncpg.Pool, job_id: str, err: Exception) -> None:
    await pool.execute(
        "UPDATE processing_jobs SET status='FAILED', error_message=$2, current_step='FAILED', finished_at=now() WHERE id=$1::uuid",
        job_id, str(err)[:1000],
    )

async def fetch_document(pool: asyncpg.Pool, doc_id: str) -> asyncpg.Record | None:
    return await pool.fetchrow(
        "SELECT source_type, content, storage_path FROM documents WHERE id=$1::uuid",
        doc_id,
    )

async def fetch_job(pool: asyncpg.Pool, job_id: str) -> asyncpg.Record | None:
    return await pool.fetchrow(
        "SELECT id, document_id, status FROM processing_jobs WHERE id=$1::uuid",
        job_id,
    )

async def replace_chunks(
    pool: asyncpg.Pool,
    doc_id: str,
    chunks,
    vectors,
    embedding_model: str | None = None,
) -> None:
    if len(chunks) != len(vectors):
        raise ValueError("chunks and vectors length mismatch")
    
    async with pool.acquire() as conn:
        async with conn.transaction():
            await conn.execute(
                "DELETE FROM document_chunks WHERE document_id=$1::uuid", doc_id
            )
            await conn.executemany(
                """INSERT INTO document_chunks
                     (document_id, chunk_index, content, start_offset, end_offset,
                      embedding, embedding_model)
                   VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)""",
                [
                    (doc_id, c.index, c.text, c.start, c.end, v, embedding_model)
                    for c, v in zip(chunks, vectors)
                ],
            )