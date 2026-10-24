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

	if err := s.repo.CreateDocumentWithJob(ctx, docId, jobId, text); err != nil {
		return "", "", err
	}

	// push job to queue
	payload, err := json.Marshal(map[string]string{
		"jobId":      jobId,
		"documentId": docId,
	})
	if err != nil {
		return "", "", err
	}
	if err := s.rdb.LPush(ctx, queueName, payload).Err(); err != nil {
		return "", "", err
	}

	return docId, jobId, nil
}

func (s *Service) GetDocument(ctx context.Context, id string) (*Document, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidID
	}

	return s.repo.GetDocument(ctx, id)
}

func (s *Service) GetJob(ctx context.Context, id string) (*Job, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidID
	}

	return s.repo.GetJob(ctx, id)
}
