package document

import (
	"encoding/json"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("POST /api/v1/document", CreateHandler(svc))
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
