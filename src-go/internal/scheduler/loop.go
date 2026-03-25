package scheduler

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
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
				log.WithError(err).Warn("scheduler loop tick failed")
			}
		}
	}
}
