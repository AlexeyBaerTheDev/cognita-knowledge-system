CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type TEXT NOT NULL, 
    original_filename TEXT,
    mime_type TEXT,
    status TEXT NOT NULL DEFAULT 'PENDING',
    title TEXT,
    language TEXT,
    content_hash TEXT,
    storage_path TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_content_hash ON documents(content_hash);

CREATE TABLE IF NOT EXISTS document_metadata (
    document_id UUID PRIMARY KEY REFERENCES documents(id) ON DELETE CASCADE,
    author TEXT,
    published_at TIMESTAMPTZ,
    source_url TEXT,
    page_count INTEGER,
    file_size BIGINT,
    custom_metadata JSONB DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    page_number INTEGER,
    start_offset INTEGER,
    end_offset INTEGER,
    embedding vector(768),
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chunks_document ON document_chunks(document_id);

CREATE INDEX IF NOT EXISTS idx_chunks_embedding ON document_chunks
USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS processing_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    current_step TEXT,
    progress INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON processing_jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_document ON processing_jobs(document_id);

CREATE TABLE IF NOT EXISTS generated_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    result_type TEXT NOT NULL,
    content TEXT,
    model TEXT,
    prompt_version TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_results_document ON generated_results(document_id);
CREATE INDEX IF NOT EXISTS idx_results_type ON generated_results(result_type);