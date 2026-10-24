package document

import (
	"context"
	"fmt"

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
