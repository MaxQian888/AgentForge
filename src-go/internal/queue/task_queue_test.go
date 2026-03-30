package queue

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestQueue(t *testing.T) (*TaskQueue, *redis.Client) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})

	return NewTaskQueue(client, "project-1"), client
}

func TestTaskQueueInitIsIdempotent(t *testing.T) {
	queue, _ := newTestQueue(t)
	ctx := context.Background()

	if err := queue.Init(ctx); err != nil {
		t.Fatalf("Init() first call error = %v", err)
	}
	if err := queue.Init(ctx); err != nil {
		t.Fatalf("Init() second call error = %v", err)
	}
}

func TestTaskQueueEnqueueDequeueAckAndLen(t *testing.T) {
	queue, client := newTestQueue(t)
	ctx := context.Background()

	if err := queue.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	message := TaskMessage{
		TaskID:     "task-1",
		Type:       "coding",
		Priority:   "high",
		AssigneeID: "member-1",
		BudgetUSD:  4.5,
		MaxTurns:   8,
		Payload:    `{"title":"Implement queue tests"}`,
	}
	messageID, err := queue.Enqueue(ctx, message)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if messageID == "" {
		t.Fatal("Enqueue() returned empty message ID")
	}

	length, err := queue.Len(ctx)
	if err != nil {
		t.Fatalf("Len() error = %v", err)
	}
	if length != 1 {
		t.Fatalf("Len() = %d, want 1", length)
	}

	tasks, err := queue.Dequeue(ctx, "coder-1", 10)
	if err != nil {
		t.Fatalf("Dequeue() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(Dequeue()) = %d, want 1", len(tasks))
	}
	if tasks[0] != message {
		t.Fatalf("Dequeue()[0] = %+v, want %+v", tasks[0], message)
	}

	pendingBeforeAck, err := client.XPending(ctx, queue.streamKey, queue.groupName).Result()
	if err != nil {
		t.Fatalf("XPending() before ack error = %v", err)
	}
	if pendingBeforeAck.Count != 1 {
		t.Fatalf("pending count before ack = %d, want 1", pendingBeforeAck.Count)
	}

	if err := queue.Ack(ctx, messageID); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}

	pendingAfterAck, err := client.XPending(ctx, queue.streamKey, queue.groupName).Result()
	if err != nil {
		t.Fatalf("XPending() after ack error = %v", err)
	}
	if pendingAfterAck.Count != 0 {
		t.Fatalf("pending count after ack = %d, want 0", pendingAfterAck.Count)
	}
}

func TestTaskQueueDequeueSkipsMalformedMessagesAndHandlesEmptyReads(t *testing.T) {
	queue, client := newTestQueue(t)
	ctx := context.Background()

	if err := queue.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if _, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: queue.streamKey,
		Values: map[string]any{
			"data": "{not-json",
		},
	}).Result(); err != nil {
		t.Fatalf("seed malformed stream entry: %v", err)
	}

	tasks, err := queue.Dequeue(ctx, "coder-2", 10)
	if err != nil {
		t.Fatalf("Dequeue() malformed error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("len(Dequeue() malformed) = %d, want 0", len(tasks))
	}

	tasks, err = queue.Dequeue(ctx, "coder-2", 10)
	if err != nil {
		t.Fatalf("Dequeue() empty error = %v", err)
	}
	if tasks != nil {
		t.Fatalf("Dequeue() empty = %+v, want nil", tasks)
	}
}
