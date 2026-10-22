import httpx

from app.config.config import get_ollama_url, get_embedding_model

EMBED_TIMEOUT = 60.0
BATCH_SIZE = 32

async def embed(texts: list[str], batch_size: int = BATCH_SIZE) -> list[list[float]]:
    if not texts:
        return []

    if batch_size <= 0:
        raise ValueError("batch_size must be positive")

    model = get_embedding_model()
    if not model:
        raise RuntimeError("embedding_model is not set")

    base_url = get_ollama_url()
    if not base_url:
        raise ValueError("ollama_url is not set")

    url = f"{base_url.rstrip('/')}/api/embed"
    vectors: list[list[float]] = []

    async with httpx.AsyncClient(timeout=EMBED_TIMEOUT) as client:
        for start in range(0, len(texts), batch_size):
            batch = texts[start:start + batch_size]

            response = await client.post(
                url,
                json={"model": model, "input": batch}
            )
            response.raise_for_status()

            data = response.json()
            embeddings = data.get("embeddings")

            if not isinstance(embeddings, list):
                raise ValueError(f"unexpected ollama response: {data}")

            if len(embeddings) != len(batch):
                raise ValueError(f"ollama returned {len(embeddings)} embeddings for {len(batch)} inputs")

            vectors.extend(embeddings)

    return vectors