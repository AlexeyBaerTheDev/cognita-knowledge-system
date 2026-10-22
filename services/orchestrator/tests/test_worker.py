import pytest

from app import worker


def test_parse_payload_returns_ids():
    job_id, document_id = worker.parse_payload(
        '{"jobId": "job-1", "documentId": "doc-1"}'
    )

    assert (job_id, document_id) == ("job-1", "doc-1")


def test_parse_payload_accepts_non_string_ids():
    job_id, document_id = worker.parse_payload(
        '{"jobId": 1, "documentId": 2}'
    )

    assert (job_id, document_id) == ("1", "2")


@pytest.mark.parametrize(
    "raw",
    [
        "not json",
        "[]",
        "null",
        '{"jobId": "job-1"}',
        '{"documentId": "doc-1"}',
        '{"jobId": "", "documentId": "doc-1"}',
    ],
)
def test_parse_payload_rejects_bad_messages(raw):
    with pytest.raises(ValueError):
        worker.parse_payload(raw)


@pytest.mark.asyncio
async def test_handle_message_processes_valid_payload(monkeypatch):
    calls = []

    async def fake_process_job(pool, job_id, document_id):
        calls.append((pool, job_id, document_id))

    monkeypatch.setattr(worker, "process_job", fake_process_job)
    pool = object()

    result = await worker.handle_message(
        pool, '{"jobId": "job-1", "documentId": "doc-1"}'
    )

    assert result is True
    assert calls == [(pool, "job-1", "doc-1")]


@pytest.mark.asyncio
async def test_handle_message_survives_job_failure(monkeypatch):
    async def failing_process_job(pool, job_id, document_id):
        raise RuntimeError("boom")

    monkeypatch.setattr(worker, "process_job", failing_process_job)

    result = await worker.handle_message(
        object(), '{"jobId": "job-1", "documentId": "doc-1"}'
    )

    assert result is False


@pytest.mark.asyncio
async def test_handle_message_rejects_broken_json(monkeypatch):
    calls = []

    async def fake_process_job(pool, job_id, document_id):
        calls.append((job_id, document_id))

    monkeypatch.setattr(worker, "process_job", fake_process_job)

    result = await worker.handle_message(object(), "{broken")

    assert result is False
    assert calls == []
