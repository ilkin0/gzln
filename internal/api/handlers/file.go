package handlers

import "net/http"

type FileHandler struct{}

func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("FIle upladed successfully"))
}
