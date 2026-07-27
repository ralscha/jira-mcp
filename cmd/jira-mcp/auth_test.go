package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{name: "valid token", authHeader: "Bearer s3cret", wantStatus: http.StatusOK},
		{name: "wrong token", authHeader: "Bearer nope", wantStatus: http.StatusUnauthorized},
		{name: "missing header", authHeader: "", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", authHeader: "Basic s3cret", wantStatus: http.StatusUnauthorized},
		{name: "token as scheme", authHeader: "s3cret", wantStatus: http.StatusUnauthorized},
	}

	handler := requireBearerToken("s3cret", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
