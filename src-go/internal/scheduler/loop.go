package scheduler

import (
	"context"
	"log/slog"
	"time"
)

func RunLoop(ctx context.Context, pollInterval time.Duration, service *Service) {
	if service == nil {
		return
	}
	if pollInterval <= 0 {
		pollInterval = 15 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := service.RunDue(ctx); err != nil {
				slog.Warn("scheduler loop tick failed", "error", err)
			}
		}
	}
}
