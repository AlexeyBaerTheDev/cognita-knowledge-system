ALTER TABLE document_chunks ADD COLUMN IF NOT EXISTS embedding_model TEXT;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS content TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS uq_chunks_doc_index
    ON document_chunks(document_id, chunk_index);

ALTER TABLE processing_jobs
    ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_attempts INT NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;