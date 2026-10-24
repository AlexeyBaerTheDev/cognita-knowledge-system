package document

import (
	"encoding/json"
	"errors"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("POST /api/v1/documents", CreateHandler(svc))
	mux.HandleFunc("GET /api/v1/documents/{id}", GetDocumentHandler(svc))
	mux.HandleFunc("GET /api/v1/jobs/{id}", GetJobHandler(svc))
}

type CreateRequest struct {
	Text string `json:"text"`
}

type CreateResponse struct {
	DocumentId string `json:"documentId"`
	JobId      string `json:"jobId"`
	Status     string `json:"status"`
}

func CreateHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.Text == "" {
			http.Error(w, "text is required", http.StatusBadRequest)
			return
		}

		docId, jobId, err := svc.CreateFromText(r.Context(), req.Text)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(CreateResponse{
			DocumentId: docId,
			JobId:      jobId,
			Status:     "PENDING",
		})
	}
}

func GetDocumentHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		doc, err := svc.GetDocument(r.Context(), r.PathValue("id"))
		switch {
		case errors.Is(err, ErrInvalidID):
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		case errors.Is(err, ErrNotFound):
			http.Error(w, "document not found", http.StatusNotFound)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}
}

func GetJobHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		job, err := svc.GetJob(r.Context(), r.PathValue("id"))
		switch {
		case errors.Is(err, ErrInvalidID):
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		case errors.Is(err, ErrNotFound):
			http.Error(w, "job not found", http.StatusNotFound)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-type", "application/json")
		json.NewEncoder(w).Encode(job)
	}

}
