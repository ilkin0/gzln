package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	json.NewEncoder(w).Encode(resp)
}

func Ok(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

func Error(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, APIResponse{
		Success: false,
		Message: msg,
	})
}

func StreamBinary(
	w http.ResponseWriter,
	r io.Reader,
	opts ...func(http.ResponseWriter),
) error {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")

	for _, opt := range opts {
		opt(w)
	}

	_, err := io.Copy(w, r)
	return err
}

func WithContentLength(n int64) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
	}
}

func WithFilename(name string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set(
			"Content-Disposition",
			fmt.Sprintf(`attachment; filename="%s"`, name),
		)
	}
}
