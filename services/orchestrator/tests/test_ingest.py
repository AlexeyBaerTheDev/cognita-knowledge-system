import pytest

from app.pipeline import ingest
from app.pipeline.chunking import Chunk


class Fakes:
    def __init__(self):
        self.calls: list[tuple] = []
        self.job = {"id": "job-1", "document_id": "doc-1", "status": "PENDING"}
        self.document = {
            "source_type": "text",
            "content": "hello world",
            "storage_path": None,
        }
        self.chunks = [Chunk(index=0, text="hello world", start=0, end=11)]
        self.vectors = [[0.1] * 768]
        self.chunk_error = None
        self.embed_error = None
        self.replace_error = None
        self.failed_err = None
        self.embedding_model = "embeddinggemma"

    def record(self, *call):
        self.calls.append(call)

    def names(self) -> list[str]:
        return [call[0] for call in self.calls]

    def statuses(self) -> list[str]:
        return [call[2] for call in self.calls if call[0] == "set_document_status"]


@pytest.fixture
def fakes(monkeypatch):
    f = Fakes()

    async def fetch_job(pool, job_id):
        f.record("fetch_job", job_id)
        return f.job

    async def fetch_document(pool, document_id):
        f.record("fetch_document", document_id)
        return f.document

    async def set_job_running(pool, job_id):
        f.record("set_job_running", job_id)

    async def set_document_status(pool, document_id, status):
        f.record("set_document_status", document_id, status)

    async def set_job_progress(pool, job_id, step, progress):
        f.record("set_job_progress", step, progress)

    async def set_job_done(pool, job_id):
        f.record("set_job_done", job_id)

    async def set_job_failed(pool, job_id, err):
        f.record("set_job_failed", job_id)
        f.failed_err = err

    def chunk_text(text):
        f.record("chunk_text", text)
        if f.chunk_error is not None:
            raise f.chunk_error
        return f.chunks

    async def embed(texts):
        f.record("embed", list(texts))
        if f.embed_error is not None:
            raise f.embed_error
        return f.vectors

    async def replace_chunks(pool, document_id, chunks, vectors, embedding_model=None):
        f.record("replace_chunks", document_id, chunks, vectors, embedding_model)
        if f.replace_error is not None:
            raise f.replace_error

    def get_embedding_model():
        return f.embedding_model

    monkeypatch.setattr(ingest, "fetch_job", fetch_job)
    monkeypatch.setattr(ingest, "fetch_document", fetch_document)
    monkeypatch.setattr(ingest, "set_job_running", set_job_running)
    monkeypatch.setattr(ingest, "set_document_status", set_document_status)
    monkeypatch.setattr(ingest, "set_job_progress", set_job_progress)
    monkeypatch.setattr(ingest, "set_job_done", set_job_done)
    monkeypatch.setattr(ingest, "set_job_failed", set_job_failed)
    monkeypatch.setattr(ingest, "chunk_text", chunk_text)
    monkeypatch.setattr(ingest, "embed", embed)
    monkeypatch.setattr(ingest, "replace_chunks", replace_chunks)
    monkeypatch.setattr(ingest, "get_embedding_model", get_embedding_model)

    return f


@pytest.mark.asyncio
async def test_process_job_happy_path(fakes):
    await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.names() == [
        "fetch_job",
        "set_job_running",
        "set_document_status",
        "fetch_document",
        "chunk_text",
        "set_job_progress",
        "embed",
        "set_job_progress",
        "replace_chunks",
        "set_job_progress",
        "set_document_status",
        "set_job_done",
    ]
    assert fakes.statuses() == ["PROCESSING", "READY"]

    steps = [call[1] for call in fakes.calls if call[0] == "set_job_progress"]
    assert steps == ["chunk", "embed", "store"]

    embed_call = next(call for call in fakes.calls if call[0] == "embed")
    assert embed_call[1] == ["hello world"]

    replace_call = next(call for call in fakes.calls if call[0] == "replace_chunks")
    assert replace_call[1] == "doc-1"
    assert replace_call[2] == fakes.chunks
    assert replace_call[3] == fakes.vectors
    assert replace_call[4] == "embeddinggemma"


@pytest.mark.asyncio
async def test_missing_job_raises_and_does_nothing(fakes):
    fakes.job = None

    with pytest.raises(ValueError, match="not found"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.names() == ["fetch_job"]


@pytest.mark.asyncio
async def test_succeeded_job_is_skipped(fakes):
    fakes.job["status"] = "SUCCEEDED"

    await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.names() == ["fetch_job"]


@pytest.mark.asyncio
async def test_job_document_mismatch_raises(fakes):
    fakes.job["document_id"] = "another-doc"

    with pytest.raises(ValueError, match="belongs to document"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.names() == ["fetch_job"]


@pytest.mark.asyncio
async def test_missing_document_marks_failed(fakes):
    fakes.document = None

    with pytest.raises(ValueError, match="not found"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.statuses() == ["PROCESSING", "FAILED"]
    assert fakes.failed_err is not None
    assert "embed" not in fakes.names()


@pytest.mark.parametrize("content", [None, "", "   \n\t "])
@pytest.mark.asyncio
async def test_empty_content_marks_failed(fakes, content):
    fakes.document["content"] = content

    with pytest.raises(ValueError, match="no content"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.statuses() == ["PROCESSING", "FAILED"]
    assert "embed" not in fakes.names()
    assert "replace_chunks" not in fakes.names()


@pytest.mark.asyncio
async def test_no_chunks_marks_failed(fakes):
    fakes.chunks = []

    with pytest.raises(ValueError, match="no chunks"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.statuses() == ["PROCESSING", "FAILED"]
    assert "embed" not in fakes.names()


@pytest.mark.asyncio
async def test_embedding_failure_keeps_old_chunks(fakes):
    fakes.embed_error = RuntimeError("ollama is down")

    with pytest.raises(RuntimeError, match="ollama is down"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.statuses() == ["PROCESSING", "FAILED"]
    assert "replace_chunks" not in fakes.names()
    assert "set_job_done" not in fakes.names()


@pytest.mark.asyncio
async def test_embedding_count_mismatch_marks_failed(fakes):
    fakes.vectors = []

    with pytest.raises(ValueError, match="mismatch"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.statuses() == ["PROCESSING", "FAILED"]
    assert "replace_chunks" not in fakes.names()


@pytest.mark.asyncio
async def test_replace_chunks_failure_marks_failed(fakes):
    fakes.replace_error = RuntimeError("db is down")

    with pytest.raises(RuntimeError, match="db is down"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.statuses() == ["PROCESSING", "FAILED"]
    assert "set_job_done" not in fakes.names()


@pytest.mark.asyncio
async def test_failed_status_update_does_not_hide_original_error(fakes, monkeypatch):
    async def broken_set_job_failed(pool, job_id, err):
        raise RuntimeError("cannot update job")

    monkeypatch.setattr(ingest, "set_job_failed", broken_set_job_failed)
    fakes.document = None

    with pytest.raises(ValueError, match="not found"):
        await ingest.process_job(object(), "job-1", "doc-1")

    assert fakes.statuses() == ["PROCESSING", "FAILED"]
