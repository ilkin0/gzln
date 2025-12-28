package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/logger"
	"github.com/ilkin0/gzln/internal/utils"
)

func (h *FileHandler) GetFileSalt(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	shareID := chi.URLParam(r, "shareID")
	ctx := context.Background()

	log.Debug("fetching file salt",
		slog.String("share_id", shareID),
	)

	fs, err := h.fileService.GetFileSalt(ctx, shareID)
	if err != nil {
		log.Warn("file salt not found",
			slog.String("share_id", shareID),
			slog.String("error", err.Error()),
		)
		utils.Error(w, http.StatusNotFound, "Not found File salt with shareID")
		return
	}

	utils.Ok(w, fs)
}
func (h *FileHandler) GetFileMetadata(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	shareID := chi.URLParam(r, "shareID")
	ctx := context.Background()

	log.Info("fetching file metadata",
		slog.String("share_id", shareID),
	)

	mdata, err := h.fileService.GetFileMetadataByShareID(ctx, shareID)
	if err != nil {
		log.Warn("file metadata not found",
			slog.String("share_id", shareID),
			slog.String("error", err.Error()),
		)
		utils.Error(w, http.StatusNotFound, "File metadata not found")
		return
	}

	utils.Ok(w, mdata)
}

func (h *ChunkHandler) DownloadChunk(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	shareID := chi.URLParam(r, "shareID")
	chunkIndexStr := chi.URLParam(r, "chunkIndex")

	chunkIndex, err := strconv.ParseInt(chunkIndexStr, 10, 32)
	if err != nil {
		log.Warn("invalid chunk index",
			slog.String("chunk_index_str", chunkIndexStr),
			slog.String("error", err.Error()),
		)
		utils.Error(w, http.StatusBadRequest, "Invalid chunk index")
		return
	}

	log.Info("downloading chunk",
		slog.String("share_id", shareID),
		slog.Int64("chunk_index", chunkIndex),
	)

	ctx := context.Background()
	chunkReader, err := h.chunkService.DownloadChunk(ctx, shareID, chunkIndex)

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

		log.Error("chunk download failed",
			slog.String("error", err.Error()),
			slog.String("share_id", shareID),
			slog.Int64("chunk_index", chunkIndex),
			slog.Int("http_status", status),
		)

		utils.Error(w, status, message)
		return
	}

	defer chunkReader.Close()

	log.Debug("streaming chunk data",
		slog.String("share_id", shareID),
		slog.Int64("chunk_index", chunkIndex),
	)

	err = utils.StreamBinary(w, chunkReader)
	if err != nil {
		log.Error("failed to stream chunk",
			slog.String("error", err.Error()),
			slog.String("share_id", shareID),
			slog.Int64("chunk_index", chunkIndex),
		)
		return
	}

	log.Info("chunk downloaded successfully",
		slog.String("share_id", shareID),
		slog.Int64("chunk_index", chunkIndex),
	)
}

func (h *FileHandler) CompleteDownload(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())
	shareID := chi.URLParam(r, "shareID")

	log.Info("completing download",
		slog.String("share_id", shareID),
	)

	ctx := context.Background()
	err := h.fileService.CompleteDownload(ctx, shareID)
	if err != nil {
		log.Error("failed to complete download",
			slog.String("error", err.Error()),
			slog.String("share_id", shareID),
		)
		utils.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Info("download completed successfully",
		slog.String("share_id", shareID),
	)

	utils.Ok(w, nil)
}
