package document

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateDocumentWithJob(ctx context.Context, docId, jobId, content string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
	INSERT INTO documents(id, source_type, content)
	VALUES ($1, $2, $3)`,
		docId, "text", content); err != nil {
		return fmt.Errorf("insert document: %w", err)
	}

	if _, err := tx.Exec(ctx, `
	INSERT INTO processing_jobs(id, document_id, job_type)
	VALUES($1, $2, $3)`,
		jobId, docId, "ingest"); err != nil {
		return fmt.Errorf("insert processing_jobs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func (r *Repository) GetDocument(ctx context.Context, docId string) (*Document, error) {
	const query = `
	SELECT id::text AS id, source_type, COALESCE(content, '') AS content, status, created_at, updated_at
	FROM documents
	WHERE id = $1::uuid;`

	rows, err := r.pool.Query(ctx, query, docId)
	if err != nil {
		return nil, fmt.Errorf("query document: %w", err)
	}

	doc, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Document])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan document: %w", err)
	}

	return &doc, nil
}

func (r *Repository) GetJob(ctx context.Context, jobId string) (*Job, error) {
	const query = `
	SELECT id::text AS id,
		       document_id::text AS document_id,
		       job_type,
		       status,
		       current_step,
		       COALESCE(progress, 0) AS progress,
		       error_message,
		       attempts,
		       max_attempts,
			   next_attempt_at,
		       created_at,
		       started_at,
		       finished_at
	FROM processing_jobs
	WHERE id = $1::uuid`

	rows, err := r.pool.Query(ctx, query, jobId)
	if err != nil {
		return nil, fmt.Errorf("query job: %w", err)
	}

	job, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Job])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan job: %w", err)
	}

	return &job, nil
}
