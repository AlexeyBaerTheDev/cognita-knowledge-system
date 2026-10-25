package document

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the database from TEST_DATABASE_URL. Integration tests
// are skipped when the variable is not set.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("ping: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

// cleanupDocument removes the document after the test. The job is removed by
// ON DELETE CASCADE.
func cleanupDocument(t *testing.T, pool *pgxpool.Pool, docID string) {
	t.Helper()

	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(),
			`DELETE FROM documents WHERE id = $1::uuid`, docID,
		); err != nil {
			t.Errorf("cleanup document %s: %v", docID, err)
		}
	})
}

func TestCreateDocumentWithJob(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	docID := uuid.NewString()
	jobID := uuid.NewString()
	cleanupDocument(t, pool, docID)

	if err := repo.CreateDocumentWithJob(ctx, docID, jobID, "hello"); err != nil {
		t.Fatalf("CreateDocumentWithJob: %v", err)
	}

	doc, err := repo.GetDocument(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.ID != docID {
		t.Errorf("document id = %q, want %q", doc.ID, docID)
	}
	if doc.Content != "hello" {
		t.Errorf("content = %q, want %q", doc.Content, "hello")
	}
	if doc.Status != "PENDING" {
		t.Errorf("status = %q, want PENDING", doc.Status)
	}

	job, err := repo.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.DocumentID == nil || *job.DocumentID != docID {
		t.Errorf("document id = %v, want %q", job.DocumentID, docID)
	}
	if job.Status != "PENDING" {
		t.Errorf("job status = %q, want PENDING", job.Status)
	}
	if job.NextAttemptAt != nil {
		t.Errorf("nextAttemptAt = %v, want nil", job.NextAttemptAt)
	}
}

func TestCreateDocumentWithJobRollsBack(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	docID := uuid.NewString()
	firstJobID := uuid.NewString()
	secondJobID := uuid.NewString()
	cleanupDocument(t, pool, docID)

	if err := repo.CreateDocumentWithJob(ctx, docID, firstJobID, "first"); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// The document id is already taken, so the second transaction must fail
	// and leave no extra job behind.
	if err := repo.CreateDocumentWithJob(ctx, docID, secondJobID, "second"); err == nil {
		t.Fatal("second create: expected error, got nil")
	}

	var jobs int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM processing_jobs WHERE document_id = $1::uuid`, docID,
	).Scan(&jobs); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if jobs != 1 {
		t.Errorf("jobs = %d, want 1", jobs)
	}
}

func TestGetDocumentNullContent(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	docID := uuid.NewString()
	cleanupDocument(t, pool, docID)

	if _, err := pool.Exec(ctx,
		`INSERT INTO documents (id, source_type, content) VALUES ($1::uuid, 'text', NULL)`,
		docID,
	); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	doc, err := repo.GetDocument(ctx, docID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.Content != "" {
		t.Errorf("content = %q, want empty string", doc.Content)
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)

	if _, err := repo.GetDocument(context.Background(), uuid.NewString()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetJobNotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)

	if _, err := repo.GetJob(context.Background(), uuid.NewString()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
