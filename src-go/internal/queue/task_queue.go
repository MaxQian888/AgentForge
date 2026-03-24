package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TaskMessage represents a task enqueued for Agent execution.
type TaskMessage struct {
	TaskID     string  `json:"task_id"`
	Type       string  `json:"type"`      // "coding", "review", "test", "decompose"
	Priority   string  `json:"priority"`  // "critical", "high", "medium", "low"
	AssigneeID string  `json:"assignee_id,omitempty"`
	BudgetUSD  float64 `json:"budget_usd"`
	MaxTurns   int     `json:"max_turns"`
	Payload    string  `json:"payload"` // JSON-encoded full task info
}

// TaskQueue manages task distribution via Redis Streams with Consumer Groups.
type TaskQueue struct {
	rdb       *redis.Client
	streamKey string
	groupName string
}

// NewTaskQueue creates a new task queue for the given project.
func NewTaskQueue(rdb *redis.Client, projectID string) *TaskQueue {
	return &TaskQueue{
		rdb:       rdb,
		streamKey: fmt.Sprintf("af:task:queue:%s", projectID),
		groupName: fmt.Sprintf("af:agent:group:%s", projectID),
	}
}

// Init creates the Consumer Group (idempotent).
func (q *TaskQueue) Init(ctx context.Context) error {
	err := q.rdb.XGroupCreateMkStream(ctx, q.streamKey, q.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group: %w", err)
	}
	return nil
}

// Enqueue adds a task message to the stream.
func (q *TaskQueue) Enqueue(ctx context.Context, msg TaskMessage) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal task message: %w", err)
	}

	id, err := q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamKey,
		Values: map[string]interface{}{
			"task_id":  msg.TaskID,
			"priority": msg.Priority,
			"type":     msg.Type,
			"data":     string(data),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("enqueue task: %w", err)
	}
	return id, nil
}

// Dequeue reads tasks from the stream using Consumer Group.
func (q *TaskQueue) Dequeue(ctx context.Context, consumerID string, count int64) ([]TaskMessage, error) {
	results, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.groupName,
		Consumer: fmt.Sprintf("agent:%s", consumerID),
		Streams:  []string{q.streamKey, ">"},
		Count:    count,
		Block:    5 * time.Second,
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("dequeue tasks: %w", err)
	}

	var tasks []TaskMessage
	for _, stream := range results {
		for _, xmsg := range stream.Messages {
			var msg TaskMessage
			if data, ok := xmsg.Values["data"].(string); ok {
				if err := json.Unmarshal([]byte(data), &msg); err != nil {
					continue
				}
			}
			tasks = append(tasks, msg)
		}
	}
	return tasks, nil
}

// Ack acknowledges processed messages.
func (q *TaskQueue) Ack(ctx context.Context, messageIDs ...string) error {
	return q.rdb.XAck(ctx, q.streamKey, q.groupName, messageIDs...).Err()
}

// Len returns the current stream length.
func (q *TaskQueue) Len(ctx context.Context) (int64, error) {
	return q.rdb.XLen(ctx, q.streamKey).Result()
}
