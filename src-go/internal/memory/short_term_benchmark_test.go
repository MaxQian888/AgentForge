package memory_test

import (
	"fmt"
	"testing"

	"github.com/react-go-quick-starter/server/internal/memory"
)

func BenchmarkShortTermMemoryStore(b *testing.B) {
	store := memory.NewShortTermMemory(memory.Config{
		MaxTokens:      4096,
		TokenEstimator: tokenEstimator,
	})

	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := store.Store(memory.StoreInput{
			Scope:   "bench-session",
			ID:      fmt.Sprintf("entry-%d", index),
			Content: "alpha beta gamma",
		}); err != nil {
			b.Fatalf("Store() error = %v", err)
		}
	}
}

func BenchmarkShortTermMemoryContext(b *testing.B) {
	store := memory.NewShortTermMemory(memory.Config{
		MaxTokens:            4096,
		DefaultContextTokens: 256,
		TokenEstimator:       tokenEstimator,
	})
	for index := 0; index < 128; index++ {
		if _, err := store.Store(memory.StoreInput{
			Scope:   "bench-session",
			ID:      fmt.Sprintf("entry-%d", index),
			Content: "alpha beta gamma",
		}); err != nil {
			b.Fatalf("Store() setup error = %v", err)
		}
	}

	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := store.Context("bench-session", 128); err != nil {
			b.Fatalf("Context() error = %v", err)
		}
	}
}
