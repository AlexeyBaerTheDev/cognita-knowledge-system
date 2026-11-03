package search

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSearcher struct {
	resp  *Response
	err   error
	query string
	limit int
	calls int
}

func (f *fakeSearcher) Search(ctx context.Context, query string, limit int) (*Response, error) {
	f.calls++
	f.query = query
	f.limit = limit
	return f.resp, f.err
}

func TestSearchHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		searcher   *fakeSearcher
		wantStatus int
		wantQuery  string
		wantLimit  int
		wantCalls  int
		wantBody   string
	}{
		{
			name: "success",
			body: `{"query":"  кошка  ","limit":3}`,
			searcher: &fakeSearcher{
				resp: &Response{
					Results: []Result{
						{
							DocumentID: "doc-1",
							ChunkIndex: 0,
							Content:    "hello",
							Score:      0.9,
						},
					},
				},
			},
			wantStatus: http.StatusOK,
			wantQuery:  "кошка",
			wantLimit:  3,
			wantCalls:  1,
			wantBody:   `"documentId":"doc-1"`,
		},
		{
			name:       "default limit",
			body:       `{"query":"кошка"}`,
			searcher:   &fakeSearcher{resp: &Response{}},
			wantStatus: http.StatusOK,
			wantQuery:  "кошка",
			wantLimit:  5,
			wantCalls:  1,
			wantBody:   `"results":[]`,
		},
		{
			name:       "empty query",
			body:       `{"query":""}`,
			searcher:   &fakeSearcher{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "query is required",
		},
		{
			name:       "whitespace query",
			body:       `{"query":"   "}`,
			searcher:   &fakeSearcher{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "query is required",
		},
		{
			name:       "limit zero",
			body:       `{"query":"кошка","limit":0}`,
			searcher:   &fakeSearcher{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "limit must be between 1 and 20",
		},
		{
			name:       "limit too big",
			body:       `{"query":"кошка","limit":21}`,
			searcher:   &fakeSearcher{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "limit must be between 1 and 20",
		},
		{
			name:       "invalid json",
			body:       `{`,
			searcher:   &fakeSearcher{},
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid json",
		},
		{
			name:       "upstream bad request",
			body:       `{"query":"кошка"}`,
			searcher:   &fakeSearcher{err: &UpstreamError{StatusCode: http.StatusBadRequest, Body: "bad query"}},
			wantStatus: http.StatusBadRequest,
			wantQuery:  "кошка",
			wantLimit:  5,
			wantCalls:  1,
			wantBody:   "bad query",
		},
		{
			name:       "upstream server error",
			body:       `{"query":"кошка"}`,
			searcher:   &fakeSearcher{err: &UpstreamError{StatusCode: http.StatusInternalServerError, Body: "boom"}},
			wantStatus: http.StatusBadGateway,
			wantQuery:  "кошка",
			wantLimit:  5,
			wantCalls:  1,
			wantBody:   "search unavailable",
		},
		{
			name:       "network error",
			body:       `{"query":"кошка"}`,
			searcher:   &fakeSearcher{err: errors.New("connection refused")},
			wantStatus: http.StatusBadGateway,
			wantQuery:  "кошка",
			wantLimit:  5,
			wantCalls:  1,
			wantBody:   "search unavailable",
		},
		{
			name:       "nil response",
			body:       `{"query":"кошка"}`,
			searcher:   &fakeSearcher{},
			wantStatus: http.StatusBadGateway,
			wantQuery:  "кошка",
			wantLimit:  5,
			wantCalls:  1,
			wantBody:   "empty response from orchestrator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			SearchHandler(tt.searcher).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.searcher.calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", tt.searcher.calls, tt.wantCalls)
			}
			if tt.wantCalls > 0 {
				if tt.searcher.query != tt.wantQuery {
					t.Errorf("query = %q, want %q", tt.searcher.query, tt.wantQuery)
				}
				if tt.searcher.limit != tt.wantLimit {
					t.Errorf("limit = %d, want %d", tt.searcher.limit, tt.wantLimit)
				}
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want substring %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestSearchHandlerSuccessResponse(t *testing.T) {
	searcher := &fakeSearcher{
		resp: &Response{
			Results: []Result{
				{
					DocumentID: "doc-1",
					ChunkIndex: 2,
					Content:    "hello",
					Score:      0.9,
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(`{"query":"кошка","limit":3}`))
	rec := httptest.NewRecorder()

	SearchHandler(searcher).ServeHTTP(rec, req)

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
	if len(got.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(got.Results))
	}
	if got.Results[0].DocumentID != "doc-1" || got.Results[0].Content != "hello" {
		t.Errorf("result = %+v", got.Results[0])
	}
}

func TestRegisterRoutes(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRoutes(mux, &fakeSearcher{resp: &Response{}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(`{"query":"кошка"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, body = %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
