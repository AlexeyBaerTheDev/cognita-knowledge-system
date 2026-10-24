package document

import (
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrInvalidID = errors.New("invalid id")
)

const (
	SourceTypeText = "text"
	JobTypeIngest  = "ingest"
)

type Document struct {
	ID         string    `json:"id"`
	SourceType string    `json:"sourceType"`
	Content    string    `json:"content"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Job struct {
	ID            string     `json:"id"`
	DocumentID    *string    `json:"documentId"`
	JobType       string     `json:"jobType"`
	Status        string     `json:"status"`
	CurrentStep   *string    `json:"currentStep"`
	Progress      int        `json:"progress"`
	ErrorMessage  *string    `json:"errorMessage"`
	Attempts      int        `json:"attempts"`
	MaxAttempts   int        `json:"maxAttempts"`
	NextAttemptAt *time.Time `json:"nextAttemptAt" db:"next_attempt_at"`
	CreatedAt     time.Time  `json:"createdAt"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}
