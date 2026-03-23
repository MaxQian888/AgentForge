package database_test

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/react-go-quick-starter/server/pkg/database"
)

func TestNewRedis_InvalidURL(t *testing.T) {
	_, err := database.NewRedis("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewRedis_UnreachableHost(t *testing.T) {
	_, err := database.NewRedis("redis://127.0.0.1:59998")
	if err == nil {
		t.Fatal("expected error for unreachable redis")
	}
}

func TestNewRedis_Success(t *testing.T) {
	s := miniredis.RunT(t)
	client, err := database.NewRedis("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	_ = client.Close()
}
