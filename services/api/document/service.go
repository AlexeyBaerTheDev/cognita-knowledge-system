package document

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const queueName = "jobs"

type Service struct {
	repo *Repository
	rdb  *redis.Client
}

func NewService(repo *Repository, rdb *redis.Client) *Service {
	return &Service{repo: repo, rdb: rdb}
}

// get text, save document, create job, push into redis and return job id and document id
func (s *Service) CreateFromText(ctx context.Context, text string) (string, string, error) {
	docId := uuid.NewString()
	jobId := uuid.NewString()

	// save document
	if err := s.repo.InsertDocuments(ctx, docId, "text", text); err != nil {
		return "", "", err
	}

	// save job
	if err := s.repo.InsertJobs(ctx, jobId, docId, "ingest"); err != nil {
		return "", "", err
	}

	// push job to queue
	payload, _ := json.Marshal(map[string]string{
		"jobId":      jobId,
		"documentId": docId,
	})
	if err := s.rdb.LPush(ctx, queueName, payload).Err(); err != nil {
		return "", "", err
	}

	return docId, jobId, nil
}
