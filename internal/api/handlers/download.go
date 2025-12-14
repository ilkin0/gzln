package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/utils"
)

func (h *FileHandler) GetFileSalt(w http.ResponseWriter, r *http.Request) {
	shareId := chi.URLParam(r, "shareId")
	ctx := context.Background()

	fs, err := h.fileService.GetFileSalt(ctx, shareId)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "Not found File salt with shareID")
		return
	}

	utils.Ok(w, fs)
}
func (h *FileHandler) GetFileMetadata(w http.ResponseWriter, r *http.Request) {
	shareId := chi.URLParam(r, "shareId")
	ctx := context.Background()

	mdata, err := h.fileService.GetFileMetadataByShareID(ctx, shareId)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "File metadata not found")
		return
	}

	utils.Ok(w, mdata)
}

func (h *ChunkHandler) DownloadChunk(w http.ResponseWriter, r *http.Request) {
	shareId := chi.URLParam(r, "shareId")
	chunkIndexStr := chi.URLParam(r, "chunkIndex")
	chunkIndex, err := strconv.ParseInt(chunkIndexStr, 10, 32)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "Invalid chunk index")
		return
	}
	ctx := context.Background()
	chunkReader, err := h.chunkService.DownloadChunk(ctx, shareId, chunkIndex)

	if err != nil {
		status := http.StatusInternalServerError
		message := "Failed to download chunk"

		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no rows"):
			status = http.StatusNotFound
			message = "File not found or has expired"
		case strings.Contains(errMsg, "limit reached"):
			status = http.StatusForbidden
			message = "Download limit reached"
		case strings.Contains(errMsg, "storage path"):
			status = http.StatusNotFound
			message = "Chunk not found"
		}

		utils.Error(w, status, message)
		return
	}

	defer chunkReader.Close()
	err = utils.StreamBinary(w, chunkReader)
	if err != nil {
		log.Printf("Failed to stream chunk %d for share %s: %v", chunkIndex, shareId, err)
		return
	}
}
