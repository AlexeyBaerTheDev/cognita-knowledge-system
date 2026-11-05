package ask

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAsker struct {
	resp     *Response
	err      error
	question string
	limit    int
	calls    int
}

func (f *fakeAsker) Ask(ctx context.Context, question string, limit int) (*Response, error) {
	f.calls++
	f.question = question
	f.limit = limit
	return f.resp, f.err
}

func TestAskHandler(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		asker        *fakeAsker
		wantStatus   int
		wantQuestion string
		wantLimit    int
		wantCalls    int
		wantBody     string
	}{
		{
			name: "success",
			body: `{"question":"  кошка  ","limit":3}`,
			asker: &fakeAsker{
				resp: &Response{
					Answer: "Кошка сидела на подоконнике [1].",
					Sources: []Source{
						{
							DocumentID: "doc-1",
							ChunkIndex: 0,
							Content:    "hello",
							Score:      0.9,
						},
					},
				},
			},
			wantStatus:   http.StatusOK,
			wantQuestion: "кошка",
			wantLimit:    3,
			wantCalls:    1,
			wantBody:     `"documentId":"doc-1"`,
		},
		{
			name:         "default limit",
			body:         `{"question":"кошка"}`,
			asker:        &fakeAsker{resp: &Response{Answer: "no data"}},
			wantStatus:   http.StatusOK,
			wantQuestion: "кошка",
			wantLimit:    5,
			wantCalls:    1,
			wantBody:     `"sources":[]`,
		},
		{
			name:       "empty question",
			body:       `{"question":""}`,
			asker:      &fakeAsker{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "question is required",
		},
		{
			name:       "whitespace question",
			body:       `{"question":"   "}`,
			asker:      &fakeAsker{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "question is required",
		},
		{
			name:       "limit zero",
			body:       `{"question":"кошка","limit":0}`,
			asker:      &fakeAsker{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "limit must be between 1 and 20",
		},
		{
			name:       "limit too big",
			body:       `{"question":"кошка","limit":21}`,
			asker:      &fakeAsker{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "limit must be between 1 and 20",
		},
		{
			name:       "invalid json",
			body:       `{`,
			asker:      &fakeAsker{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid json",
		},
		{
			name:         "upstream bad request",
			body:         `{"question":"кошка"}`,
			asker:        &fakeAsker{err: &UpstreamError{StatusCode: http.StatusBadRequest, Body: "bad question"}},
			wantStatus:   http.StatusBadRequest,
			wantQuestion: "кошка",
			wantLimit:    5,
			wantCalls:    1,
			wantBody:     "bad question",
		},
		{
			name:         "upstream server error",
			body:         `{"question":"кошка"}`,
			asker:        &fakeAsker{err: &UpstreamError{StatusCode: http.StatusInternalServerError, Body: "boom"}},
			wantStatus:   http.StatusBadGateway,
			wantQuestion: "кошка",
			wantLimit:    5,
			wantCalls:    1,
			wantBody:     "ask unavailable",
		},
		{
			name:         "network error",
			body:         `{"question":"кошка"}`,
			asker:        &fakeAsker{err: errors.New("connection refused")},
			wantStatus:   http.StatusBadGateway,
			wantQuestion: "кошка",
			wantLimit:    5,
			wantCalls:    1,
			wantBody:     "ask unavailable",
		},
		{
			name:         "nil response",
			body:         `{"question":"кошка"}`,
			asker:        &fakeAsker{},
			wantStatus:   http.StatusBadGateway,
			wantQuestion: "кошка",
			wantLimit:    5,
			wantCalls:    1,
			wantBody:     "empty response from orchestrator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/ask", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			AskHandler(tt.asker).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.asker.calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", tt.asker.calls, tt.wantCalls)
			}
			if tt.wantCalls > 0 {
				if tt.asker.question != tt.wantQuestion {
					t.Errorf("question = %q, want %q", tt.asker.question, tt.wantQuestion)
				}
				if tt.asker.limit != tt.wantLimit {
					t.Errorf("limit = %d, want %d", tt.asker.limit, tt.wantLimit)
				}
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want substring %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestAskHandlerSuccessResponse(t *testing.T) {
	asker := &fakeAsker{
		resp: &Response{
			Answer: "Кошка сидела на подоконнике [1].",
			Sources: []Source{
				{
					DocumentID: "doc-1",
					ChunkIndex: 2,
					Content:    "hello",
					Score:      0.9,
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ask", strings.NewReader(`{"question":"кошка","limit":3}`))
	rec := httptest.NewRecorder()

	AskHandler(asker).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var got Response
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Answer != "Кошка сидела на подоконнике [1]." {
		t.Errorf("answer = %q", got.Answer)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(got.Sources))
	}
	if got.Sources[0].DocumentID != "doc-1" || got.Sources[0].Content != "hello" {
		t.Errorf("source = %+v", got.Sources[0])
	}
}

func TestAskRegisterRoutes(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, &fakeAsker{resp: &Response{Answer: "ok"}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ask", strings.NewReader(`{"question":"кошка"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/ask", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
