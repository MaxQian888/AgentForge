package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type MemberRepository struct {
	db *gorm.DB
}

func NewMemberRepository(db *gorm.DB) *MemberRepository {
	return &MemberRepository{db: db}
}

func (r *MemberRepository) Create(ctx context.Context, member *model.Member) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newMemberRecord(member)).Error; err != nil {
		return fmt.Errorf("create member: %w", err)
	}
	return nil
}

func (r *MemberRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Member, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record memberRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get member by id: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

func (r *MemberRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Member, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []memberRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	members := make([]*model.Member, 0, len(records))
	for i := range records {
		members = append(members, records[i].toModel())
	}
	return members, nil
}

func (r *MemberRepository) ListAll(ctx context.Context) ([]*model.Member, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var records []memberRecord
	if err := r.db.WithContext(ctx).
		Order("created_at").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list all members: %w", err)
	}

	members := make([]*model.Member, 0, len(records))
	for i := range records {
		members = append(members, records[i].toModel())
	}
	return members, nil
}

func (r *MemberRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateMemberRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.Status != nil {
		status := model.NormalizeMemberStatus(*req.Status, req.IsActive != nil && *req.IsActive)
		updates["status"] = status
		updates["is_active"] = model.IsMemberStatusActive(status)
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.IMPlatform != nil {
		updates["im_platform"] = *req.IMPlatform
	}
	if req.IMUserID != nil {
		updates["im_user_id"] = *req.IMUserID
	}
	if req.AgentConfig != nil {
		updates["agent_config"] = newJSONText(*req.AgentConfig, "{}")
	}
	if req.Skills != nil {
		updates["skills"] = newStringList(*req.Skills)
	}
	if req.IsActive != nil && req.Status == nil {
		updates["is_active"] = *req.IsActive
		updates["status"] = model.NormalizeMemberStatus("", *req.IsActive)
	}
	if len(updates) == 0 {
		return nil
	}

	if err := r.db.WithContext(ctx).
		Model(&memberRecord{}).
		Where("id = ?", id).
		Updates(updates).
		Error; err != nil {
		return fmt.Errorf("update member: %w", err)
	}
	return nil
}

func (r *MemberRepository) BulkUpdateStatus(
	ctx context.Context,
	projectID uuid.UUID,
	memberIDs []uuid.UUID,
	status string,
) ([]model.BulkUpdateMemberResult, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	normalizedStatus := model.NormalizeMemberStatus(status, status == model.MemberStatusActive)
	now := time.Now().UTC()

	var records []memberRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND id IN ?", projectID, memberIDs).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list members for bulk update: %w", err)
	}

	recordMap := make(map[uuid.UUID]memberRecord, len(records))
	for _, record := range records {
		recordMap[record.ID] = record
	}

	results := make([]model.BulkUpdateMemberResult, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		record, ok := recordMap[memberID]
		if !ok {
			results = append(results, model.BulkUpdateMemberResult{
				MemberID: memberID.String(),
				Success:  false,
				Error:    "member not found in project",
			})
			continue
		}

		err := r.db.WithContext(ctx).
			Model(&memberRecord{}).
			Where("id = ? AND project_id = ?", memberID, projectID).
			Updates(map[string]any{
				"status":     normalizedStatus,
				"is_active":  model.IsMemberStatusActive(normalizedStatus),
				"updated_at": now,
			}).Error
		if err != nil {
			results = append(results, model.BulkUpdateMemberResult{
				MemberID: memberID.String(),
				Success:  false,
				Error:    err.Error(),
			})
			continue
		}

		record.Status = normalizedStatus
		record.IsActive = model.IsMemberStatusActive(normalizedStatus)
		record.UpdatedAt = now
		recordMap[memberID] = record
		results = append(results, model.BulkUpdateMemberResult{
			MemberID: memberID.String(),
			Success:  true,
			Status:   normalizedStatus,
		})
	}

	return results, nil
}

func (r *MemberRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Delete(&memberRecord{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete member: %w", err)
	}
	return nil
}

func (r *MemberRepository) GetByUserAndProject(ctx context.Context, userID, projectID uuid.UUID) (*model.Member, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record memberRecord
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND project_id = ?", userID, projectID).
		Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get member by user and project: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}
