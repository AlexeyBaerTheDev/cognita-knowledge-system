package document

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestGetDocumentHandlerInvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()

	GetDocumentHandler(&Service{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetJobHandlerInvalidID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/not-a-uuid", nil)
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()

	GetJobHandler(&Service{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateHandlerInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	CreateHandler(&Service{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateHandlerEmptyText(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", strings.NewReader(`{"text":""}`))
	rec := httptest.NewRecorder()

	CreateHandler(&Service{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetDocumentHandlerOK(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	ctx := context.Background()

	docID := uuid.NewString()
	jobID := uuid.NewString()
	cleanupDocument(t, pool, docID)

	if err := repo.CreateDocumentWithJob(ctx, docID, jobID, "handler content"); err != nil {
		t.Fatalf("create: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+docID, nil)
	req.SetPathValue("id", docID)
	rec := httptest.NewRecorder()

	GetDocumentHandler(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var doc Document
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if doc.ID != docID {
		t.Errorf("id = %q, want %q", doc.ID, docID)
	}
	if doc.Content != "handler content" {
		t.Errorf("content = %q, want %q", doc.Content, "handler content")
	}
}

func TestGetDocumentHandlerNotFound(t *testing.T) {
	pool := testPool(t)
	svc := NewService(NewRepository(pool), nil)

	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()

	GetDocumentHandler(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetJobHandlerOK(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	svc := NewService(repo, nil)
	ctx := context.Background()

	docID := uuid.NewString()
	jobID := uuid.NewString()
	cleanupDocument(t, pool, docID)

	if err := repo.CreateDocumentWithJob(ctx, docID, jobID, "job content"); err != nil {
		t.Fatalf("create: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	req.SetPathValue("id", jobID)
	rec := httptest.NewRecorder()

	GetJobHandler(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var job Job
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if job.ID != jobID {
		t.Errorf("id = %q, want %q", job.ID, jobID)
	}
	if job.DocumentID == nil || *job.DocumentID != docID {
		t.Errorf("document id = %v, want %q", job.DocumentID, docID)
	}
}

func TestGetJobHandlerNotFound(t *testing.T) {
	pool := testPool(t)
	svc := NewService(NewRepository(pool), nil)

	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+id, nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()

	GetJobHandler(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
