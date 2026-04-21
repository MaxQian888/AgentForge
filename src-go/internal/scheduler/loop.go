package scheduler

import (
	"context"
	"time"

	applog "github.com/agentforge/server/internal/log"
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
			tickCtx := ctx
			if applog.TraceID(tickCtx) == "" {
				tickCtx = applog.WithTrace(tickCtx, applog.NewTraceID())
				log.WithFields(log.Fields{"trace_id": applog.TraceID(tickCtx), "origin": "scheduler.tick"}).Info("trace.generated_for_background_job")
			}
			if _, err := service.RunDue(tickCtx); err != nil {
				log.WithError(err).Warn("scheduler loop tick failed")
			}
		}
	}
}
