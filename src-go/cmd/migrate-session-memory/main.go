package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/config"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/pkg/database"
)

func main() {
	projectIDFlag := flag.String("project-id", "", "Target project UUID for imported episodic memories")
	roleIDFlag := flag.String("role-id", "", "Optional role ID to attach to imported memories")
	scopeFlag := flag.String("scope", model.MemoryScopeProject, "Memory scope for imported snapshots (global, project, role)")
	dirFlag := flag.String("snapshot-dir", "", "Directory containing persisted bridge session snapshots (*.json)")
	flag.Parse()

	if strings.TrimSpace(*projectIDFlag) == "" || strings.TrimSpace(*dirFlag) == "" {
		fmt.Fprintln(os.Stderr, "project-id and snapshot-dir are required")
		os.Exit(1)
	}

	projectID, err := uuid.Parse(strings.TrimSpace(*projectIDFlag))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid project-id: %v\n", err)
		os.Exit(1)
	}

	cfg := config.Load()
	if strings.TrimSpace(cfg.PostgresURL) == "" {
		fmt.Fprintln(os.Stderr, "POSTGRES_URL is required")
		os.Exit(1)
	}

	db, err := database.NewPostgres(cfg.PostgresURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = database.ClosePostgres(db)
	}()

	repo := repository.NewAgentMemoryRepository(db)
	svc := service.NewEpisodicMemoryService(repo)
	imported, err := svc.ImportSessionSnapshots(context.Background(), service.SessionSnapshotImportRequest{
		ProjectID: projectID,
		RoleID:    strings.TrimSpace(*roleIDFlag),
		Scope:     strings.TrimSpace(*scopeFlag),
		Dir:       strings.TrimSpace(*dirFlag),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate session snapshots: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("imported %d session snapshots into episodic memory\n", imported)
}
