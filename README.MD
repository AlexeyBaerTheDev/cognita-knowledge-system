# Cognita

A local knowledge system: it accepts text, splits it into chunks asynchronously, computes embeddings, and stores everything in PostgreSQL with pgvector — so it can later be searched by meaning.

> Current status: **MVP ingestion**. Text upload and vector index construction work end-to-end. Search and RAG answers are in progress.

---

## Contents

- [Features](#features)
- [Architecture](#architecture)
- [Stack](#stack)
- [Quick start](#quick-start)
- [Environment variables](#environment-variables)
- [API](#api)
- [How ingestion works](#how-ingestion-works)
- [Data model](#data-model)
- [Project structure](#project-structure)
- [Development](#development)
- [Tests](#tests)
- [Limitations](#limitations)
- [Roadmap](#roadmap)
- [Troubleshooting](#troubleshooting)

---

## Features

- accept text over HTTP;
- create a document and a processing job;
- process the job asynchronously in a separate worker;
- split text into overlapping chunks;
- compute embeddings through Ollama (`embeddinggemma`, 768 dimensions);
- store chunks and vectors in PostgreSQL + pgvector;
- track status and progress;
- handle failures correctly: the document and the job both become `FAILED`.

Not available yet: search, LLM answers, file/PDF upload, retries, and authentication.

---

## Architecture

```
┌──────────┐   HTTP    ┌──────────────────┐
│ Client   │ ────────► │  Go API :8080    │
└──────────┘           └────────┬─────────┘
                                │ INSERT documents / processing_jobs
                                │ LPUSH jobs
                                ▼
                       ┌─────────────────┐
                       │ Redis  "jobs"   │
                       └────────┬────────┘
                                │ BRPOP
                                ▼
                       ┌─────────────────┐
                       │ Worker          │
                       │ python -m       │
                       │ app.worker      │
                       └───┬─────────┬───┘
                           │         │
                 ┌─────────▼──┐  ┌───▼────────────┐
                 │ PostgreSQL │  │ Ollama         │
                 │ + pgvector │  │ embeddinggemma │
                 └────────────┘  └────────────────┘

┌──────────────────┐
│ Orchestrator     │  FastAPI :8000 — health/ready,
│ uvicorn app.main │  future /search and /ask
└──────────────────┘
```

Core principle: **the API does not process documents**. It only creates records and pushes a job into the queue. Processing is done by a separate worker process that reuses the same `app/` modules.

---

## Stack

| Layer | Technology |
|---|---|
| API | Go 1.26, `net/http`, `pgx/v5`, `go-redis/v9` |
| Orchestrator | Python 3.11+, FastAPI, asyncpg, redis-py |
| Queue | Redis 7 (`LPUSH` + `BRPOP`) |
| Database | PostgreSQL 16 + pgvector (`vector(768)`, HNSW) |
| Embeddings | Ollama, `embeddinggemma` |
| LLM (planned) | Ollama, `qwen3:8b` |
| Infrastructure | Docker Compose |

---

## Quick start

### Requirements

- Docker and Docker Compose
- Free ports: `5432`, `6379`, `8080`, `8000`, `11434`

### 1. Configuration

```bash
cp .env.example .env
```

Fill in `.env`:

```dotenv
POSTGRES_DB=knowledge
POSTGRES_USER=postgres
POSTGRES_PASSWORD=change-me

DATABASE_URL=postgres://postgres:change-me@database:5432/knowledge?sslmode=disable
REDIS_URL=redis://cache:6379

OLLAMA_URL=http://ollama:11434
EMBEDDING_MODEL=embeddinggemma
LLM_MODEL=qwen3:8b

API_PORT=8080
```

> Inside Docker the hostnames are `database`, `cache`, and `ollama`.
> The local `make run-server` / `make run-worker` targets replace them with `localhost`.

### 2. Start the stack

```bash
make build      # build api, orchestrator, worker
make up         # start all services
make migrate    # apply migrations
make pull-models
```

`make pull-models` downloads `embeddinggemma` and `qwen3:8b` — this may take a few minutes.

### 3. Verify

```bash
curl http://localhost:8080/health   # Go API
curl http://localhost:8080/ready    # Postgres + Redis
curl http://localhost:8000/health   # Orchestrator
curl http://localhost:8000/ready    # Ollama + Redis
```

### 4. Create the first document

```bash
curl -s -X POST http://localhost:8080/api/v1/document \
  -H 'Content-Type: application/json' \
  -d '{"text": "The cat sat on the windowsill and watched the birds. It was raining outside."}'
```

Response `202 Accepted`:

```json
{
  "documentId": "f4589005-673b-42d1-a750-d26324c1f367",
  "jobId": "4be26ed0-646f-4b2a-9f5d-612a16ae2d9e",
  "status": "PENDING"
}
```

After a few seconds:

```bash
make db-shell
```

```sql
SELECT id, status FROM documents ORDER BY created_at DESC LIMIT 3;
SELECT status, current_step, progress FROM processing_jobs ORDER BY created_at DESC LIMIT 3;
SELECT chunk_index, vector_dims(embedding), embedding_model
FROM document_chunks ORDER BY created_at DESC LIMIT 5;
```

Expected: `documents.status = 'READY'`, `progress = 100`, `vector_dims = 768`.

---

## Environment variables

| Variable | Purpose | Example |
|---|---|---|
| `POSTGRES_DB` | database name | `knowledge` |
| `POSTGRES_USER` | Postgres user | `postgres` |
| `POSTGRES_PASSWORD` | Postgres password | `change-me` |
| `DATABASE_URL` | DSN for Go and Python | `postgres://...@database:5432/knowledge` |
| `REDIS_URL` | Redis address | `redis://cache:6379` |
| `OLLAMA_URL` | Ollama address | `http://ollama:11434` |
| `EMBEDDING_MODEL` | embedding model | `embeddinggemma` |
| `LLM_MODEL` | generation model (not used yet) | `qwen3:8b` |
| `API_PORT` | Go API port | `8080` |
| `WORKER_PORT` | reserved | `8000` |
| `UPLOAD_DIR` | file storage directory (future) | `/data/uploads` |

`.env` is listed in `.gitignore`. Only `.env.example` is committed.

---

## API

### Go API (`:8080`)

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | liveness, always `200` |
| `GET` | `/ready` | checks Postgres and Redis |
| `POST` | `/api/v1/document` | create a document from text |

#### `POST /api/v1/document`

Body:

```json
{ "text": "Document text" }
```

Response `202`:

```json
{
  "documentId": "uuid",
  "jobId": "uuid",
  "status": "PENDING"
}
```

Errors:

- `400` — invalid JSON or empty `text`;
- `500` — database or Redis error.

> The path is singular (`/document`) for now. Renaming it to `/documents` is on the roadmap.

### Orchestrator (`:8000`)

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | liveness |
| `GET` | `/ready` | checks Ollama and Redis |

Search (`/search`) and RAG (`/ask`) are not implemented yet.

---

## How ingestion works

1. The Go API receives text and inserts records:
   - `documents` → `PENDING`;
   - `processing_jobs` → `PENDING`.
2. The API runs `LPUSH jobs` with `{jobId, documentId}` and returns `202`.
3. The worker runs `BRPOP jobs` (FIFO) and calls `process_job`.
4. `process_job`:
   1. checks whether the job has already succeeded;
   2. sets the job to `RUNNING` and the document to `PROCESSING`;
   3. reads `documents.content`;
   4. splits the text into chunks (1000 characters, 150 overlap);
   5. computes embeddings in batches of 32;
   6. atomically replaces the chunks (`DELETE` + `INSERT` in one transaction);
   7. sets the document to `READY` and the job to `SUCCEEDED`.
5. On any error, both the document and the job become `FAILED`, and `error_message` records the reason.

### Statuses

```
documents:        PENDING → PROCESSING → READY
                              └────────► FAILED

processing_jobs:  PENDING → RUNNING → SUCCEEDED
                              └──────► FAILED
```

### Guarantees

- `replace_chunks` runs in a transaction: readers see either the old chunk set or the new one.
- Embeddings are computed **before** old chunks are deleted: if Ollama fails, the previous index is preserved.
- The document status is updated before the job status: if the process crashes between the two, the document is already searchable and the job can be reprocessed.

---

## Data model

| Table | Purpose |
|---|---|
| `documents` | document: `source_type`, `content`, `status`, metadata |
| `document_metadata` | author, date, URL, page count |
| `document_chunks` | chunks: `content`, `start_offset`, `end_offset`, `embedding vector(768)` |
| `processing_jobs` | jobs: `status`, `current_step`, `progress`, `attempts` |
| `generated_results` | generation results (future) |

Indexes:

- `idx_chunks_embedding` — HNSW, `vector_cosine_ops`;
- `uq_chunks_doc_index` — uniqueness of `(document_id, chunk_index)`;
- indexes on statuses and `document_id`.

---

## Project structure

```
cognita/
├── docker-compose.yaml
├── Makefile
├── .env.example
├── README.MD
├── services/
│   ├── api/                        # Go
│   │   ├── cmd/server/main.go
│   │   ├── db/postgres.go
│   │   ├── document/               # handler, service, repository
│   │   ├── queue/redis.go
│   │   └── migrations/
│   │       ├── 001_init.sql
│   │       └── 002_add_content.sql
│   └── orchestrator/               # Python
│       ├── app/
│       │   ├── config/config.py
│       │   ├── db/db.py
│       │   ├── ollama/ollama.py
│       │   ├── pipeline/
│       │   │   ├── chunking.py
│       │   │   └── ingest.py
│       │   ├── queue/redis.py
│       │   ├── main.py             # FastAPI
│       │   └── worker.py           # worker entrypoint
│       ├── tests/
│       ├── Dockerfile
│       └── pyproject.toml
└── data/uploads/                   # file volume (future)
```

---

## Development

### Python

```bash
python3 -m venv .venv
source .venv/bin/activate
cd services/orchestrator
pip install -e ".[dev]"
```

Run the worker on the host (Postgres/Redis/Ollama containers must be running):

```bash
make run-worker
```

### Go

```bash
cd services/api
go build ./...
go test ./...
```

Run the API on the host:

```bash
make run-server
```

### Useful commands

| Command | Description |
|---|---|
| `make up` | start the stack |
| `make down` | stop the stack (data is preserved) |
| `make build` | rebuild api, orchestrator, worker |
| `make logs` | logs for all services |
| `make logs-worker` | worker logs |
| `make migrate` | apply migrations |
| `make db-shell` | psql inside the container |
| `make db-tables` | list tables |
| `make pull-models` | download Ollama models |
| `make test` | Go + Python tests |
| `make run-server` | run the Go API locally |
| `make run-worker` | run the worker locally |

---

## Tests

```bash
make test
```

Or separately:

```bash
cd services/orchestrator
../../.venv/bin/python -m pytest
```

Current state: **55 Python tests**.

| File | Coverage |
|---|---|
| `test_chunking.py` | splitting, overlap, offsets, invalid parameters |
| `test_ollama.py` | batching, response errors, live Ollama check |
| `test_queue.py` | serialization, FIFO, length, ping, close |
| `test_ingest.py` | step order, all error paths, idempotency |
| `test_worker.py` | payload parsing, broken JSON, job failure |

Integration tests are marked with `@pytest.mark.integration` and require running Redis/Ollama:

```bash
pytest -m "not integration"
```

Queue tests use a separate `jobs:test` key so they do not compete with a live worker.

---

## Limitations

- Only plain text is accepted. PDF, OCR, URLs, and files come later.
- No `/search` or `/ask`.
- The queue is **at-most-once**: `BRPOP` removes a job before processing. If the worker crashes, the job is lost.
- The `attempts`, `max_attempts`, and `next_attempt_at` fields exist in the schema, but retries are not implemented.
- The worker processes one job at a time.
- No authentication or access control.
- Migrations are not applied automatically on startup.
- The document creation path is `/api/v1/document` (singular).
- Statuses are stored as `TEXT` without `CHECK` constraints.

---

## Roadmap

### Phase B — read state

- `GET /api/v1/jobs/{id}` — status, progress, error;
- `GET /api/v1/documents/{id}` — status and metadata;
- transaction for creating `document + job`;
- `jobid` → `jobId`, `/document` → `/documents`.

### Phase C — search

- `embed(query)` → `ORDER BY embedding <=> $1` → top-k chunks;
- `POST /search` in Python, a thin proxy in Go;
- inspect the plan with `EXPLAIN ANALYZE`.

### Phase D — RAG

- `POST /ask`: search → prompt with numbered fragments → `qwen3:8b`;
- answers with sources;
- an honest “no data” when nothing is found.

### Phase E — files

- upload, `storage_path`, PDF parsing, OCR, URL import;
- a separate `job_type` for text extraction.

### Phase F — reliability

- `BRPOPLPUSH` + `jobs:processing` + acknowledgement;
- retries with `attempts` and `next_attempt_at`;
- per-document locking;
- metrics and structured logging;
- worker health check.

---

## Troubleshooting

### `lookup database ... no such host`

The `database` container must be on the `ko-networks` network. The `database` service in `docker-compose.yaml` must include:

```yaml
networks:
  - ko-networks
```

If the network gets out of sync:

```bash
docker compose down
docker compose up -d
```

### Worker logs `TimeoutError ... cache:6379`

This was a collision between the client-side socket timeout and `BRPOP`. The client is created with `socket_timeout=None`, so the server controls blocking. If the error persists, make sure the container is built from the current code:

```bash
docker compose up -d --build worker
```

### `model not found` in Ollama

```bash
make pull-models
```

Or manually:

```bash
docker compose exec ollama ollama pull embeddinggemma
docker compose exec ollama ollama pull qwen3:8b
```

### `ready` returns `not ready`

```bash
docker compose ps
docker compose logs database
docker compose logs cache
docker compose logs ollama
```

Check `DATABASE_URL` and `REDIS_URL`: inside Docker use service names, on the host use `localhost`.

### `make test` cannot find pytest

You need a virtual environment at the repository root:

```bash
python3 -m venv .venv
source .venv/bin/activate
cd services/orchestrator
pip install -e ".[dev]"
```

### A document is stuck in `PROCESSING`

The worker most likely crashed after `BRPOP`. Check:

```bash
make logs-worker
```

```sql
SELECT * FROM processing_jobs WHERE status = 'RUNNING';
```

Reprocess it manually or wait for the reliable queue implementation (Phase F).

---

## License

Educational project. No license specified.
