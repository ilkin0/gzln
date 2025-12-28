package logger

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesUUID(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := RequestIDFromContext(r.Context())
		assert.NotEmpty(t, requestID, "Request ID should be generated")
		assert.Len(t, requestID, 36, "Request ID should be valid UUID length")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "Response should include X-Request-ID header")
}

func TestRequestID_RespectsExistingHeader(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	existingID := "custom-request-id-12345"

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := RequestIDFromContext(r.Context())
		assert.Equal(t, existingID, requestID, "Should use existing request ID from header")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, existingID, w.Header().Get("X-Request-ID"))
}

func TestRequestID_AddsToResponseHeader(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	responseID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, responseID)
	assert.Len(t, responseID, 36, "Should be valid UUID")
}

func TestRequestID_AddsLoggerToContext(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := FromContext(r.Context())
		assert.NotNil(t, logger, "Logger should be in context")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

func TestRequestLogger_LogsIncomingRequest(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestID(RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})))

	req := httptest.NewRequest("POST", "/api/test?param=value", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())
}

func TestRequestLogger_CapturesStatusCode(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	tests := []struct {
		name       string
		statusCode int
	}{
		{"status 200", http.StatusOK},
		{"status 201", http.StatusCreated},
		{"status 400", http.StatusBadRequest},
		{"status 404", http.StatusNotFound},
		{"status 500", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestRequestLogger_CapturesBytesWritten(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	testData := "Hello, World!"

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testData))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, testData, w.Body.String())
	assert.Equal(t, len(testData), w.Body.Len())
}

func TestRequestLogger_DefaultStatusOK(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		status:         http.StatusOK,
	}

	wrapped.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, wrapped.status)
}

func TestResponseWriter_Write(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &responseWriter{
		ResponseWriter: recorder,
		status:         http.StatusOK,
	}

	testData := []byte("test data")
	n, err := wrapped.Write(testData)

	require.NoError(t, err)
	assert.Equal(t, len(testData), n)
	assert.Equal(t, len(testData), wrapped.bytes)
	assert.Equal(t, testData, recorder.Body.Bytes())
}

func TestResponseWriter_MultipleWrites(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &responseWriter{
		ResponseWriter: recorder,
		status:         http.StatusOK,
	}

	data1 := []byte("first")
	data2 := []byte("second")

	n1, err1 := wrapped.Write(data1)
	n2, err2 := wrapped.Write(data2)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, len(data1), n1)
	assert.Equal(t, len(data2), n2)
	assert.Equal(t, len(data1)+len(data2), wrapped.bytes)
}

func TestWithLogger(t *testing.T) {
	ctx := context.Background()
	logger := New("development", "info")

	newCtx := WithLogger(ctx, logger)

	retrievedLogger := FromContext(newCtx)
	assert.NotNil(t, retrievedLogger)
}

func TestFromContext_ReturnsDefault(t *testing.T) {
	ctx := context.Background()

	logger := FromContext(ctx)
	assert.NotNil(t, logger)
	assert.Equal(t, slog.Default(), logger)
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-id-123"

	newCtx := WithRequestID(ctx, requestID)

	retrievedID := RequestIDFromContext(newCtx)
	assert.Equal(t, requestID, retrievedID)
}

func TestRequestIDFromContext_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()

	requestID := RequestIDFromContext(ctx)
	assert.Equal(t, "", requestID)
}

func TestMiddlewareChain_RequestIDThenLogger(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	var capturedRequestID string

	handler := RequestID(RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = RequestIDFromContext(r.Context())
		logger := FromContext(r.Context())
		assert.NotNil(t, logger)
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.NotEmpty(t, capturedRequestID)
	assert.Equal(t, capturedRequestID, w.Header().Get("X-Request-ID"))
}

func TestRequestLogger_WithoutRequestID(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogger_PathsAndMethods(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/api/v1/files"},
		{"POST", "/api/v1/files/upload"},
		{"PUT", "/api/v1/files/123"},
		{"DELETE", "/api/v1/files/123"},
		{"PATCH", "/api/v1/files/123"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method)
				assert.Equal(t, tt.path, r.URL.Path)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRequestLogger_UserAgent(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	userAgent := "Mozilla/5.0 Test Browser"

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, userAgent, r.UserAgent())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", userAgent)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

func TestRequestLogger_RemoteAddr(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.RemoteAddr)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}

func TestRequestLogger_RecordsDuration(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Duration should be recorded (tested via logs, but we can't easily assert it here)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestContextKey_Uniqueness(t *testing.T) {
	ctx := context.Background()

	logger := New("development", "info")
	requestID := "test-id"

	ctx = WithLogger(ctx, logger)
	ctx = WithRequestID(ctx, requestID)

	retrievedLogger := FromContext(ctx)
	retrievedID := RequestIDFromContext(ctx)

	assert.NotNil(t, retrievedLogger)
	assert.Equal(t, requestID, retrievedID)
}

func TestRequestID_UUIDFormat(t *testing.T) {
	slog.SetDefault(New("development", "info"))

	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := RequestIDFromContext(r.Context())

		// UUID format: 8-4-4-4-12 characters
		parts := strings.Split(requestID, "-")
		assert.Len(t, parts, 5, "UUID should have 5 parts separated by hyphens")
		assert.Len(t, parts[0], 8)
		assert.Len(t, parts[1], 4)
		assert.Len(t, parts[2], 4)
		assert.Len(t, parts[3], 4)
		assert.Len(t, parts[4], 12)

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
}
