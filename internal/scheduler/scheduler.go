package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/ilkin0/gzln/internal/service"
)

type Scheduler struct {
	cleanupService *service.CleanupService
	interval       time.Duration
}

func New(cleanupService *service.CleanupService, interval time.Duration) *Scheduler {
	return &Scheduler{
		cleanupService: cleanupService,
		interval:       interval,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("scheduler started", slog.Duration("interval", s.interval))
	go s.runCleanupJob(ctx)
}

func (s *Scheduler) runCleanupJob(ctx context.Context) {
	s.executeCleanup(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeCleanup(ctx)
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) executeCleanup(ctx context.Context) {
	deleted, err := s.cleanupService.CleanupExpiredFiles(ctx)
	if err != nil {
		slog.Error("cleanup job failed", slog.String("error", err.Error()))
		return
	}

	if deleted > 0 {
		slog.Info("cleanup job completed", slog.Int("deleted_files", deleted))
	}
}
