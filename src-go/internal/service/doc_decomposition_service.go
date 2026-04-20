package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type docDecompositionTaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	CreateChildren(ctx context.Context, inputs []model.TaskChildInput) ([]*model.Task, error)
}

type docDecompositionWikiRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.WikiPage, error)
}

type docDecompositionSpaceReader interface {
	GetSpaceByID(ctx context.Context, id uuid.UUID) (*model.WikiSpace, error)
}

type DocDecompositionService struct {
	tasks docDecompositionTaskRepository
	pages docDecompositionWikiRepository
	links entityLinkRepository
}

func NewDocDecompositionService(tasks docDecompositionTaskRepository, pages docDecompositionWikiRepository, links entityLinkRepository) *DocDecompositionService {
	return &DocDecompositionService{tasks: tasks, pages: pages, links: links}
}

func (s *DocDecompositionService) DecomposeTasksFromBlocks(ctx context.Context, projectID uuid.UUID, pageID uuid.UUID, blockIDs []string, parentTaskID *uuid.UUID, createdBy *uuid.UUID) (*model.DecomposeTasksFromPageResponse, error) {
	page, err := s.pages.GetByID(ctx, pageID)
	if err != nil {
		return nil, ErrWikiPageNotFound
	}
	belongs, err := s.pageBelongsToProject(ctx, page, projectID)
	if err != nil {
		return nil, err
	}
	if !belongs {
		return nil, ErrWikiPageNotFound
	}
	blocks := extractSelectedBlocks(page.Content, blockIDs)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no matching blocks found")
	}

	createdTasks := make([]*model.Task, 0, len(blocks))
	if parentTaskID != nil {
		inputs := make([]model.TaskChildInput, 0, len(blocks))
		for _, block := range blocks {
			inputs = append(inputs, model.TaskChildInput{
				ParentID:    *parentTaskID,
				ProjectID:   projectID,
				ReporterID:  createdBy,
				Title:       block.title,
				Description: block.body,
				Priority:    "medium",
				Labels:      []string{"source:wiki"},
				BudgetUSD:   0,
			})
		}
		createdTasks, err = s.tasks.CreateChildren(ctx, inputs)
		if err != nil {
			return nil, fmt.Errorf("create child tasks from doc blocks: %w", err)
		}
	} else {
		for _, block := range blocks {
			task := &model.Task{
				ID:          uuid.New(),
				ProjectID:   projectID,
				Title:       block.title,
				Description: block.body,
				Status:      model.TaskStatusInbox,
				Priority:    "medium",
				ReporterID:  createdBy,
				Labels:      []string{"source:wiki"},
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}
			if err := s.tasks.Create(ctx, task); err != nil {
				return nil, fmt.Errorf("create task from doc block: %w", err)
			}
			createdTasks = append(createdTasks, task)
		}
	}

	for index, task := range createdTasks {
		link := &model.EntityLink{
			ID:            uuid.New(),
			ProjectID:     projectID,
			SourceType:    model.EntityTypeTask,
			SourceID:      task.ID,
			TargetType:    model.EntityTypeWikiPage,
			TargetID:      pageID,
			LinkType:      model.EntityLinkTypeRequirement,
			AnchorBlockID: &blocks[index].id,
			CreatedBy:     valueOrUUID(createdBy),
			CreatedAt:     time.Now().UTC(),
		}
		if err := s.links.Create(ctx, link); err != nil {
			return nil, fmt.Errorf("create requirement link for decomposed task: %w", err)
		}
	}

	response := &model.DecomposeTasksFromPageResponse{
		PageID:   pageID.String(),
		BlockIDs: append([]string(nil), blockIDs...),
		Tasks:    make([]model.TaskDTO, 0, len(createdTasks)),
	}
	for _, task := range createdTasks {
		response.Tasks = append(response.Tasks, task.ToDTO())
	}
	return response, nil
}

type selectedWikiBlock struct {
	id    string
	title string
	body  string
}

func extractSelectedBlocks(raw string, blockIDs []string) []selectedWikiBlock {
	if len(blockIDs) == 0 {
		return nil
	}
	var nodes []map[string]any
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		return nil
	}
	seen := make(map[string]struct{}, len(blockIDs))
	wanted := make(map[string]struct{}, len(blockIDs))
	for _, blockID := range blockIDs {
		wanted[strings.TrimSpace(blockID)] = struct{}{}
	}
	result := make([]selectedWikiBlock, 0, len(blockIDs))
	for _, node := range nodes {
		blockID, _ := node["id"].(string)
		if _, ok := wanted[blockID]; !ok {
			continue
		}
		if _, ok := seen[blockID]; ok {
			continue
		}
		seen[blockID] = struct{}{}
		text := extractWikiBlockText(node)
		if text == "" {
			text = blockID
		}
		title := text
		if len(title) > 80 {
			title = title[:80]
		}
		result = append(result, selectedWikiBlock{
			id:    blockID,
			title: title,
			body:  text,
		})
	}
	return result
}

func extractWikiBlockText(node map[string]any) string {
	switch content := node["content"].(type) {
	case string:
		return strings.TrimSpace(content)
	case []any:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := entry["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	default:
		return strings.TrimSpace(fmt.Sprint(content))
	}
}

func valueOrUUID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func (s *DocDecompositionService) pageBelongsToProject(ctx context.Context, page *model.WikiPage, projectID uuid.UUID) (bool, error) {
	if page == nil {
		return false, nil
	}
	spaceReader, ok := s.pages.(docDecompositionSpaceReader)
	if !ok {
		return true, nil
	}
	space, err := spaceReader.GetSpaceByID(ctx, page.SpaceID)
	if err != nil {
		if err == ErrWikiSpaceNotFound {
			return false, nil
		}
		return false, err
	}
	if space == nil {
		return false, nil
	}
	return space.ProjectID == projectID, nil
}
