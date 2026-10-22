from fastapi import FastAPI
import httpx
import traceback
from app.queue.redis import get_redis_client, ping, close
import app.config.config as config

app = FastAPI()

@app.get("/health")
def health():
    return {"status": "ok"}

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