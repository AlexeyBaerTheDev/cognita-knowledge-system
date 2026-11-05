import httpx

from app.config.config import get_ollama_url, get_embedding_model, get_llm_model

EMBED_TIMEOUT = 60.0
BATCH_SIZE = 32
LLM_TIMEOUT = 120.0

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

async def chat(messages: list[dict[str, str]], model: str | None = None,) -> str:
    ollama_llm_model = get_llm_model()
    ollama_url = get_ollama_url()

    if not ollama_llm_model:
        raise RuntimeError("llm_model is not set")
    if not ollama_url:
        raise ValueError("ollama_url is not set")
    if not messages:
        raise ValueError("messages is empty")

    url = f"{ollama_url.rstrip('/')}/api/chat"

    async with httpx.AsyncClient(timeout=LLM_TIMEOUT) as client:
        response = await client.post(
            url,
            json = {
                "model":    model or ollama_llm_model,
                "messages": messages,
                "stream":   False,
            },
        )
        response.raise_for_status()
        data = response.json()

    if not isinstance(data, dict):
        raise ValueError(f"unexpected ollama response: {data}")

    message = data.get("message")
    if not isinstance(message, dict):
        raise ValueError(f"unexpected ollama response: {data}")

    content = message.get("content")
    if not isinstance(content, str) or not content.strip():
        raise ValueError(f"unexpected ollama response: {data}")

    return content.strip()