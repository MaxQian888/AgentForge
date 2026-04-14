package handler

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type dispatchAttemptReader interface {
	ListByTaskID(ctx context.Context, taskID uuid.UUID, limit int) ([]*model.DispatchAttempt, error)
	ListByProjectID(ctx context.Context, projectID uuid.UUID, limit int) ([]*model.DispatchAttempt, error)
}

type dispatchQueueStatsReader interface {
	CountQueuedByProject(ctx context.Context, projectID uuid.UUID) (int, error)
	ListRecentByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]*model.AgentPoolQueueEntry, error)
}

type DispatchStatsResponse struct {
	Outcomes                  map[string]int `json:"outcomes"`
	BlockedReasons            map[string]int `json:"blockedReasons"`
	QueueDepth                int            `json:"queueDepth"`
	MedianWaitSeconds         *float64       `json:"medianWaitSeconds,omitempty"`
	PromotionSuccessRate      *float64       `json:"promotionSuccessRate,omitempty"`
	CancelledWithoutPromotion int            `json:"cancelledWithoutPromotion"`
	TerminalPromotionFailures int            `json:"terminalPromotionFailures"`
}

type DispatchStatsHandler struct {
	attempts dispatchAttemptReader
	queue    dispatchQueueStatsReader
}

func NewDispatchStatsHandler(attempts dispatchAttemptReader, queue dispatchQueueStatsReader) *DispatchStatsHandler {
	return &DispatchStatsHandler{attempts: attempts, queue: queue}
}

func (h *DispatchStatsHandler) Get(c echo.Context) error {
	since, until, err := parseDispatchStatsWindow(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	projectID := appMiddleware.GetProjectID(c)
	attempts, err := h.attempts.ListByProjectID(c.Request().Context(), projectID, 500)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetVelocityStats)
	}
	queueDepth, err := h.queue.CountQueuedByProject(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetVelocityStats)
	}
	queueEntries, err := h.queue.ListRecentByProject(c.Request().Context(), projectID, 200)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGetVelocityStats)
	}

	response := DispatchStatsResponse{
		Outcomes:       map[string]int{},
		BlockedReasons: map[string]int{},
		QueueDepth:     queueDepth,
	}
	for _, attempt := range filterDispatchAttemptsWindow(attempts, since, until) {
		if attempt == nil {
			continue
		}
		response.Outcomes[attempt.Outcome]++
		if attempt.Outcome == model.DispatchStatusBlocked {
			key := attempt.GuardrailType
			if key == "" {
				key = "unknown"
			}
			response.BlockedReasons[key]++
		}
	}
	filteredQueueEntries := filterQueueEntriesWindow(queueEntries, since, until)
	if median := computeMedianQueueWait(filteredQueueEntries); median != nil {
		response.MedianWaitSeconds = median
	}
	response.CancelledWithoutPromotion = countQueueEntriesByStatus(filteredQueueEntries, model.AgentPoolQueueStatusCancelled)
	response.TerminalPromotionFailures = countTerminalPromotionFailures(filteredQueueEntries)
	if rate := computePromotionSuccessRate(filteredQueueEntries); rate != nil {
		response.PromotionSuccessRate = rate
	}
	return c.JSON(http.StatusOK, response)
}

type DispatchHistoryHandler struct {
	attempts dispatchAttemptReader
}

func NewDispatchHistoryHandler(attempts dispatchAttemptReader) *DispatchHistoryHandler {
	return &DispatchHistoryHandler{attempts: attempts}
}

func (h *DispatchHistoryHandler) Get(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	attempts, err := h.attempts.ListByTaskID(c.Request().Context(), taskID, 100)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchTask)
	}
	history := make([]model.DispatchAttemptDTO, 0, len(attempts))
	for _, attempt := range attempts {
		history = append(history, attempt.ToDTO())
	}
	return c.JSON(http.StatusOK, history)
}

func computeMedianQueueWait(entries []*model.AgentPoolQueueEntry) *float64 {
	waits := make([]float64, 0)
	for _, entry := range entries {
		if entry == nil || entry.Status != model.AgentPoolQueueStatusPromoted {
			continue
		}
		waitSeconds := entry.UpdatedAt.Sub(entry.CreatedAt).Seconds()
		if waitSeconds >= 0 {
			waits = append(waits, waitSeconds)
		}
	}
	if len(waits) == 0 {
		return nil
	}
	sort.Float64s(waits)
	mid := len(waits) / 2
	median := waits[mid]
	if len(waits)%2 == 0 {
		median = (waits[mid-1] + waits[mid]) / 2
	}
	return &median
}

func parseDispatchStatsWindow(c echo.Context) (*time.Time, *time.Time, error) {
	parse := func(raw string) (*time.Time, error) {
		if raw == "" {
			return nil, nil
		}
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, err
		}
		return &value, nil
	}

	since, err := parse(c.QueryParam("since"))
	if err != nil {
		return nil, nil, err
	}
	until, err := parse(c.QueryParam("until"))
	if err != nil {
		return nil, nil, err
	}
	return since, until, nil
}

func filterDispatchAttemptsWindow(attempts []*model.DispatchAttempt, since *time.Time, until *time.Time) []*model.DispatchAttempt {
	if since == nil && until == nil {
		return attempts
	}
	filtered := make([]*model.DispatchAttempt, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt == nil {
			continue
		}
		if since != nil && attempt.CreatedAt.Before(*since) {
			continue
		}
		if until != nil && attempt.CreatedAt.After(*until) {
			continue
		}
		filtered = append(filtered, attempt)
	}
	return filtered
}

func filterQueueEntriesWindow(entries []*model.AgentPoolQueueEntry, since *time.Time, until *time.Time) []*model.AgentPoolQueueEntry {
	if since == nil && until == nil {
		return entries
	}
	filtered := make([]*model.AgentPoolQueueEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if since != nil && entry.UpdatedAt.Before(*since) {
			continue
		}
		if until != nil && entry.UpdatedAt.After(*until) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func countQueueEntriesByStatus(entries []*model.AgentPoolQueueEntry, status model.AgentPoolQueueStatus) int {
	count := 0
	for _, entry := range entries {
		if entry == nil || entry.Status != status {
			continue
		}
		count++
	}
	return count
}

func countTerminalPromotionFailures(entries []*model.AgentPoolQueueEntry) int {
	count := 0
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if entry.Status == model.AgentPoolQueueStatusFailed || entry.RecoveryDisposition == model.QueueRecoveryDispositionTerminal {
			count++
		}
	}
	return count
}

func computePromotionSuccessRate(entries []*model.AgentPoolQueueEntry) *float64 {
	promoted := countQueueEntriesByStatus(entries, model.AgentPoolQueueStatusPromoted)
	cancelled := countQueueEntriesByStatus(entries, model.AgentPoolQueueStatusCancelled)
	terminal := countTerminalPromotionFailures(entries)
	total := promoted + cancelled + terminal
	if total == 0 {
		return nil
	}
	rate := float64(promoted) / float64(total)
	return &rate
}
