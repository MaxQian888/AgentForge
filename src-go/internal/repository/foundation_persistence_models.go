package repository

import (
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type customFieldDefinitionRecord struct {
	ID        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	Name      string     `gorm:"column:name;not null"`
	FieldType string     `gorm:"column:field_type;not null"`
	Options   jsonText   `gorm:"column:options;type:jsonb"`
	SortOrder int        `gorm:"column:sort_order"`
	Required  bool       `gorm:"column:required"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt *time.Time `gorm:"column:deleted_at"`
}

func (customFieldDefinitionRecord) TableName() string { return "custom_field_defs" }

func newCustomFieldDefinitionRecord(def *model.CustomFieldDefinition) *customFieldDefinitionRecord {
	if def == nil {
		return nil
	}
	return &customFieldDefinitionRecord{
		ID:        def.ID,
		ProjectID: def.ProjectID,
		Name:      def.Name,
		FieldType: def.FieldType,
		Options:   newJSONText(def.Options, "[]"),
		SortOrder: def.SortOrder,
		Required:  def.Required,
		CreatedAt: def.CreatedAt,
		UpdatedAt: def.UpdatedAt,
		DeletedAt: cloneTimePointer(def.DeletedAt),
	}
}

func (r *customFieldDefinitionRecord) toModel() *model.CustomFieldDefinition {
	if r == nil {
		return nil
	}
	return &model.CustomFieldDefinition{
		ID:        r.ID,
		ProjectID: r.ProjectID,
		Name:      r.Name,
		FieldType: r.FieldType,
		Options:   r.Options.String("[]"),
		SortOrder: r.SortOrder,
		Required:  r.Required,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		DeletedAt: cloneTimePointer(r.DeletedAt),
	}
}

type customFieldValueRecord struct {
	ID         uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	TaskID     uuid.UUID `gorm:"column:task_id;type:uuid;not null;uniqueIndex:idx_custom_field_value_task_field"`
	FieldDefID uuid.UUID `gorm:"column:field_def_id;type:uuid;not null;uniqueIndex:idx_custom_field_value_task_field"`
	Value      jsonText  `gorm:"column:value;type:jsonb"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null"`
}

func (customFieldValueRecord) TableName() string { return "custom_field_values" }

func newCustomFieldValueRecord(value *model.CustomFieldValue) *customFieldValueRecord {
	if value == nil {
		return nil
	}
	return &customFieldValueRecord{
		ID:         value.ID,
		TaskID:     value.TaskID,
		FieldDefID: value.FieldDefID,
		Value:      newJSONText(value.Value, "null"),
		CreatedAt:  value.CreatedAt,
		UpdatedAt:  value.UpdatedAt,
	}
}

func (r *customFieldValueRecord) toModel() *model.CustomFieldValue {
	if r == nil {
		return nil
	}
	return &model.CustomFieldValue{
		ID:         r.ID,
		TaskID:     r.TaskID,
		FieldDefID: r.FieldDefID,
		Value:      r.Value.String("null"),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

type savedViewRecord struct {
	ID         uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID  uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	Name       string     `gorm:"column:name;not null"`
	OwnerID    *uuid.UUID `gorm:"column:owner_id;type:uuid"`
	IsDefault  bool       `gorm:"column:is_default"`
	SharedWith jsonText   `gorm:"column:shared_with;type:jsonb"`
	Config     jsonText   `gorm:"column:config;type:jsonb"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt  *time.Time `gorm:"column:deleted_at"`
}

func (savedViewRecord) TableName() string { return "saved_views" }

func newSavedViewRecord(view *model.SavedView) *savedViewRecord {
	if view == nil {
		return nil
	}
	return &savedViewRecord{
		ID:         view.ID,
		ProjectID:  view.ProjectID,
		Name:       view.Name,
		OwnerID:    cloneUUIDPointer(view.OwnerID),
		IsDefault:  view.IsDefault,
		SharedWith: newJSONText(view.SharedWith, "{}"),
		Config:     newJSONText(view.Config, "{}"),
		CreatedAt:  view.CreatedAt,
		UpdatedAt:  view.UpdatedAt,
		DeletedAt:  cloneTimePointer(view.DeletedAt),
	}
}

func (r *savedViewRecord) toModel() *model.SavedView {
	if r == nil {
		return nil
	}
	return &model.SavedView{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
		Name:       r.Name,
		OwnerID:    cloneUUIDPointer(r.OwnerID),
		IsDefault:  r.IsDefault,
		SharedWith: r.SharedWith.String("{}"),
		Config:     r.Config.String("{}"),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
		DeletedAt:  cloneTimePointer(r.DeletedAt),
	}
}

type formDefinitionRecord struct {
	ID             uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	Name           string     `gorm:"column:name;not null"`
	Slug           string     `gorm:"column:slug;not null"`
	Fields         jsonText   `gorm:"column:fields;type:jsonb"`
	TargetStatus   string     `gorm:"column:target_status"`
	TargetAssignee *uuid.UUID `gorm:"column:target_assignee;type:uuid"`
	IsPublic       bool       `gorm:"column:is_public"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
}

func (formDefinitionRecord) TableName() string { return "form_definitions" }

func newFormDefinitionRecord(form *model.FormDefinition) *formDefinitionRecord {
	if form == nil {
		return nil
	}
	return &formDefinitionRecord{
		ID:             form.ID,
		ProjectID:      form.ProjectID,
		Name:           form.Name,
		Slug:           form.Slug,
		Fields:         newJSONText(form.Fields, "[]"),
		TargetStatus:   form.TargetStatus,
		TargetAssignee: cloneUUIDPointer(form.TargetAssignee),
		IsPublic:       form.IsPublic,
		CreatedAt:      form.CreatedAt,
		UpdatedAt:      form.UpdatedAt,
		DeletedAt:      cloneTimePointer(form.DeletedAt),
	}
}

func (r *formDefinitionRecord) toModel() *model.FormDefinition {
	if r == nil {
		return nil
	}
	return &model.FormDefinition{
		ID:             r.ID,
		ProjectID:      r.ProjectID,
		Name:           r.Name,
		Slug:           r.Slug,
		Fields:         r.Fields.String("[]"),
		TargetStatus:   r.TargetStatus,
		TargetAssignee: cloneUUIDPointer(r.TargetAssignee),
		IsPublic:       r.IsPublic,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		DeletedAt:      cloneTimePointer(r.DeletedAt),
	}
}

type formSubmissionRecord struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	FormID      uuid.UUID `gorm:"column:form_id;type:uuid;not null"`
	TaskID      uuid.UUID `gorm:"column:task_id;type:uuid;not null"`
	SubmittedBy string    `gorm:"column:submitted_by"`
	SubmittedAt time.Time `gorm:"column:submitted_at;not null"`
	IPAddress   string    `gorm:"column:ip_address"`
}

func (formSubmissionRecord) TableName() string { return "form_submissions" }

func newFormSubmissionRecord(submission *model.FormSubmission) *formSubmissionRecord {
	if submission == nil {
		return nil
	}
	return &formSubmissionRecord{
		ID:          submission.ID,
		FormID:      submission.FormID,
		TaskID:      submission.TaskID,
		SubmittedBy: submission.SubmittedBy,
		SubmittedAt: submission.SubmittedAt,
		IPAddress:   submission.IPAddress,
	}
}

func (r *formSubmissionRecord) toModel() *model.FormSubmission {
	if r == nil {
		return nil
	}
	return &model.FormSubmission{
		ID:          r.ID,
		FormID:      r.FormID,
		TaskID:      r.TaskID,
		SubmittedBy: r.SubmittedBy,
		SubmittedAt: r.SubmittedAt,
		IPAddress:   r.IPAddress,
	}
}

type automationRuleRecord struct {
	ID         uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID  uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	Name       string     `gorm:"column:name;not null"`
	Enabled    bool       `gorm:"column:enabled"`
	EventType  string     `gorm:"column:event_type;not null"`
	Conditions jsonText   `gorm:"column:conditions;type:jsonb"`
	Actions    jsonText   `gorm:"column:actions;type:jsonb"`
	CreatedBy  uuid.UUID  `gorm:"column:created_by;type:uuid;not null"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt  *time.Time `gorm:"column:deleted_at"`
}

func (automationRuleRecord) TableName() string { return "automation_rules" }

func newAutomationRuleRecord(rule *model.AutomationRule) *automationRuleRecord {
	if rule == nil {
		return nil
	}
	return &automationRuleRecord{
		ID:         rule.ID,
		ProjectID:  rule.ProjectID,
		Name:       rule.Name,
		Enabled:    rule.Enabled,
		EventType:  rule.EventType,
		Conditions: newJSONText(rule.Conditions, "[]"),
		Actions:    newJSONText(rule.Actions, "[]"),
		CreatedBy:  rule.CreatedBy,
		CreatedAt:  rule.CreatedAt,
		UpdatedAt:  rule.UpdatedAt,
		DeletedAt:  cloneTimePointer(rule.DeletedAt),
	}
}

func (r *automationRuleRecord) toModel() *model.AutomationRule {
	if r == nil {
		return nil
	}
	return &model.AutomationRule{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
		Name:       r.Name,
		Enabled:    r.Enabled,
		EventType:  r.EventType,
		Conditions: r.Conditions.String("[]"),
		Actions:    r.Actions.String("[]"),
		CreatedBy:  r.CreatedBy,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
		DeletedAt:  cloneTimePointer(r.DeletedAt),
	}
}

type automationLogRecord struct {
	ID          uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	RuleID      uuid.UUID  `gorm:"column:rule_id;type:uuid;not null"`
	TaskID      *uuid.UUID `gorm:"column:task_id;type:uuid"`
	EventType   string     `gorm:"column:event_type;not null"`
	TriggeredAt time.Time  `gorm:"column:triggered_at;not null"`
	Status      string     `gorm:"column:status;not null"`
	Detail      jsonText   `gorm:"column:detail;type:jsonb"`
}

func (automationLogRecord) TableName() string { return "automation_logs" }

func newAutomationLogRecord(log *model.AutomationLog) *automationLogRecord {
	if log == nil {
		return nil
	}
	return &automationLogRecord{
		ID:          log.ID,
		RuleID:      log.RuleID,
		TaskID:      cloneUUIDPointer(log.TaskID),
		EventType:   log.EventType,
		TriggeredAt: log.TriggeredAt,
		Status:      log.Status,
		Detail:      newJSONText(log.Detail, "{}"),
	}
}

func (r *automationLogRecord) toModel() *model.AutomationLog {
	if r == nil {
		return nil
	}
	return &model.AutomationLog{
		ID:          r.ID,
		RuleID:      r.RuleID,
		TaskID:      cloneUUIDPointer(r.TaskID),
		EventType:   r.EventType,
		TriggeredAt: r.TriggeredAt,
		Status:      r.Status,
		Detail:      r.Detail.String("{}"),
	}
}

type dashboardConfigRecord struct {
	ID        uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	Name      string     `gorm:"column:name;not null"`
	Layout    jsonText   `gorm:"column:layout;type:jsonb"`
	CreatedBy uuid.UUID  `gorm:"column:created_by;type:uuid;not null"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt *time.Time `gorm:"column:deleted_at"`
}

func (dashboardConfigRecord) TableName() string { return "dashboard_configs" }

func newDashboardConfigRecord(config *model.DashboardConfig) *dashboardConfigRecord {
	if config == nil {
		return nil
	}
	return &dashboardConfigRecord{
		ID:        config.ID,
		ProjectID: config.ProjectID,
		Name:      config.Name,
		Layout:    newJSONText(config.Layout, "[]"),
		CreatedBy: config.CreatedBy,
		CreatedAt: config.CreatedAt,
		UpdatedAt: config.UpdatedAt,
		DeletedAt: cloneTimePointer(config.DeletedAt),
	}
}

func (r *dashboardConfigRecord) toModel() *model.DashboardConfig {
	if r == nil {
		return nil
	}
	return &model.DashboardConfig{
		ID:        r.ID,
		ProjectID: r.ProjectID,
		Name:      r.Name,
		Layout:    r.Layout.String("[]"),
		CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		DeletedAt: cloneTimePointer(r.DeletedAt),
	}
}

type dashboardWidgetRecord struct {
	ID          uuid.UUID `gorm:"column:id;type:uuid;primaryKey"`
	DashboardID uuid.UUID `gorm:"column:dashboard_id;type:uuid;not null"`
	WidgetType  string    `gorm:"column:widget_type;not null"`
	Config      jsonText  `gorm:"column:config;type:jsonb"`
	Position    jsonText  `gorm:"column:position;type:jsonb"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (dashboardWidgetRecord) TableName() string { return "dashboard_widgets" }

func newDashboardWidgetRecord(widget *model.DashboardWidget) *dashboardWidgetRecord {
	if widget == nil {
		return nil
	}
	return &dashboardWidgetRecord{
		ID:          widget.ID,
		DashboardID: widget.DashboardID,
		WidgetType:  widget.WidgetType,
		Config:      newJSONText(widget.Config, "{}"),
		Position:    newJSONText(widget.Position, "{}"),
		CreatedAt:   widget.CreatedAt,
		UpdatedAt:   widget.UpdatedAt,
	}
}

func (r *dashboardWidgetRecord) toModel() *model.DashboardWidget {
	if r == nil {
		return nil
	}
	return &model.DashboardWidget{
		ID:          r.ID,
		DashboardID: r.DashboardID,
		WidgetType:  r.WidgetType,
		Config:      r.Config.String("{}"),
		Position:    r.Position.String("{}"),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type milestoneRecord struct {
	ID          uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID   uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	Name        string     `gorm:"column:name;not null"`
	TargetDate  *time.Time `gorm:"column:target_date"`
	Status      string     `gorm:"column:status;not null"`
	Description string     `gorm:"column:description"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
}

func (milestoneRecord) TableName() string { return "milestones" }

func newMilestoneRecord(milestone *model.Milestone) *milestoneRecord {
	if milestone == nil {
		return nil
	}
	return &milestoneRecord{
		ID:          milestone.ID,
		ProjectID:   milestone.ProjectID,
		Name:        milestone.Name,
		TargetDate:  cloneTimePointer(milestone.TargetDate),
		Status:      milestone.Status,
		Description: milestone.Description,
		CreatedAt:   milestone.CreatedAt,
		UpdatedAt:   milestone.UpdatedAt,
		DeletedAt:   cloneTimePointer(milestone.DeletedAt),
	}
}

func (r *milestoneRecord) toModel() *model.Milestone {
	if r == nil {
		return nil
	}
	return &model.Milestone{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		TargetDate:  cloneTimePointer(r.TargetDate),
		Status:      r.Status,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		DeletedAt:   cloneTimePointer(r.DeletedAt),
	}
}
