package document

import (
	"context"
	"errors"
	"testing"
)

func TestGetDocumentInvalidID(t *testing.T) {
	svc := &Service{}

	if _, err := svc.GetDocument(context.Background(), "not-a-uuid"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("err = %v, want ErrInvalidID", err)
	}
}

func TestGetJobInvalidID(t *testing.T) {
	svc := &Service{}

	if _, err := svc.GetJob(context.Background(), "not-a-uuid"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("err = %v, want ErrInvalidID", err)
	}
}
