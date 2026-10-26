from fastapi import FastAPI, HTTPException
import httpx
import traceback
from app.queue.redis import get_redis_client, ping, close
from contextlib import asynccontextmanager
from app.db.db import (
    create_pool,
    search_chunks,
    )
from app.ollama.ollama import embed
from pydantic import BaseModel
import app.config.config as config

@asynccontextmanager
async def lifespan(app):
    pool = await create_pool(config.get_database_url())
    app.state.pool = pool
    try:
        yield
    finally:
        await pool.close()

app = FastAPI(lifespan=lifespan)

@app.get("/health")
def health():
    return {"status": "ok"}

class SearchRequest(BaseModel):
    query: str
    limit: int = 5

@app.post("/search")
async def search(req: SearchRequest):
    query = req.query.strip()
    if not query:
        raise HTTPException(400, "query is required")
    if not 1 <= req.limit <= 20:
        raise HTTPException(400, "limit must be between 1 and 20")

    vectors = await embed([query])
    rows = await search_chunks

    return {"search": [
        {
            "documentId": row["document_id"],
            "chunkIndex": row["chunk_index"],
            "content":    row["content"],
            "score":      float(row["score"])
        }
        for row in rows
    ]}

@app.get("/ready")
async def ready():
    checks = {}

    ollama_url = config.get_ollama_url()

    # Ollama
    try:
        async with httpx.AsyncClient(timeout=2.0) as client:
            r = await client.get(f"{ollama_url}/api/tags")
            r.raise_for_status()
        checks["ollama"] = "ok"
    except Exception as ex:
        checks["ollama"] = f"error: {ex}"
        traceback.print_exc()

    # Redis
    try:
        r = get_redis_client()
        await r.ping()
        await r.aclose()
        checks["redis"] = "ok"
    except Exception as ex:
        checks["redis"] = f"error: {ex}"
        traceback.print_exc()

    all_ok = all(v == "ok" for v in checks.values())

    return {
        "status": "ready" if all_ok else "not ready", "checks": checks
    }