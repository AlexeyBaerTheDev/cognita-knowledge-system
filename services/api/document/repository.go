package document

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Creating record in documents
func (r *Repository) InsertDocuments(ctx context.Context, id, sourceType, content string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO documents(id, source_type, content, status)
		VALUES ($1, $2, $3, 'PENDING')
	`, id, sourceType, content)

	return err
}

// Creating record in processing_jobs table
func (r *Repository) InsertJobs(ctx context.Context, id, documentId, jobType string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO processing_jobs(id, document_id, job_type, status)
		VALUES ($1, $2, $3, 'PENDING')
	`, id, documentId, jobType)

	return err
}
