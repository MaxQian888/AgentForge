package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWorkflowConfigToDTOAndParseTransitions(t *testing.T) {
	createdAt := time.Date(2026, 3, 30, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(2 * time.Hour)
	workflowID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	projectID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	config := &WorkflowConfig{
		ID:          workflowID,
		ProjectID:   projectID,
		Transitions: json.RawMessage(`{"todo":["doing","done"]}`),
		Triggers:    json.RawMessage(`[{"fromStatus":"todo","toStatus":"doing","action":"notify"}]`),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	dto := config.ToDTO()
	if dto.ID != workflowID.String() {
		t.Fatalf("dto.ID = %q, want %q", dto.ID, workflowID.String())
	}
	if !reflect.DeepEqual(dto.Transitions, map[string][]string{"todo": {"doing", "done"}}) {
		t.Fatalf("dto.Transitions = %#v", dto.Transitions)
	}
	if len(dto.Triggers) != 1 || dto.Triggers[0].Action != "notify" {
		t.Fatalf("dto.Triggers = %#v", dto.Triggers)
	}
	if dto.CreatedAt != createdAt.Format(time.RFC3339) {
		t.Fatalf("dto.CreatedAt = %q, want %q", dto.CreatedAt, createdAt.Format(time.RFC3339))
	}

	parsed, err := config.ParseTransitions()
	if err != nil {
		t.Fatalf("ParseTransitions() error = %v", err)
	}
	if !reflect.DeepEqual(parsed, map[string][]string{"todo": {"doing", "done"}}) {
		t.Fatalf("ParseTransitions() = %#v", parsed)
	}

	empty := (&WorkflowConfig{}).ToDTO()
	if empty.Transitions == nil || len(empty.Transitions) != 0 {
		t.Fatalf("empty.Transitions = %#v, want empty map", empty.Transitions)
	}
	if empty.Triggers == nil || len(empty.Triggers) != 0 {
		t.Fatalf("empty.Triggers = %#v, want empty slice", empty.Triggers)
	}

	if parsed, err := (&WorkflowConfig{}).ParseTransitions(); err != nil || parsed != nil {
		t.Fatalf("empty ParseTransitions() = %#v, %v; want nil, nil", parsed, err)
	}
}

func TestSavedViewShareConfigAccessAndDTO(t *testing.T) {
	viewID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	projectID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	ownerID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	memberID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	now := time.Date(2026, 3, 30, 9, 30, 0, 0, time.UTC)
	deletedAt := now.Add(30 * time.Minute)

	view := &SavedView{
		ID:         viewID,
		ProjectID:  projectID,
		Name:       "Planner",
		OwnerID:    &ownerID,
		IsDefault:  true,
		SharedWith: `{"roleIds":["reviewer"],"memberIds":["` + memberID.String() + `"]}`,
		Config:     `{"layout":"board"}`,
		CreatedAt:  now,
		UpdatedAt:  now.Add(10 * time.Minute),
		DeletedAt:  &deletedAt,
	}

	shareConfig := view.ShareConfig()
	if !reflect.DeepEqual(shareConfig.RoleIDs, []string{"reviewer"}) {
		t.Fatalf("ShareConfig().RoleIDs = %#v", shareConfig.RoleIDs)
	}
	if !reflect.DeepEqual(shareConfig.MemberIDs, []string{memberID.String()}) {
		t.Fatalf("ShareConfig().MemberIDs = %#v", shareConfig.MemberIDs)
	}

	if !view.IsAccessibleTo(ownerID, nil) {
		t.Fatal("owner should have access")
	}
	if !view.IsAccessibleTo(memberID, nil) {
		t.Fatal("shared member should have access")
	}
	if !view.IsAccessibleTo(uuid.New(), []string{"reviewer"}) {
		t.Fatal("shared role should have access")
	}
	if view.IsAccessibleTo(uuid.New(), []string{"guest"}) {
		t.Fatal("unshared user should not have access")
	}

	dto := view.ToDTO()
	if dto.OwnerID == nil || *dto.OwnerID != ownerID.String() {
		t.Fatalf("dto.OwnerID = %#v, want %q", dto.OwnerID, ownerID.String())
	}
	if string(dto.SharedWith) != `{"roleIds":["reviewer"],"memberIds":["`+memberID.String()+`"]}` {
		t.Fatalf("dto.SharedWith = %s", string(dto.SharedWith))
	}
	if string(dto.Config) != `{"layout":"board"}` {
		t.Fatalf("dto.Config = %s", string(dto.Config))
	}
	if dto.DeletedAt == nil || *dto.DeletedAt != deletedAt.Format(time.RFC3339) {
		t.Fatalf("dto.DeletedAt = %#v", dto.DeletedAt)
	}

	publicView := &SavedView{}
	if !publicView.IsAccessibleTo(uuid.New(), nil) {
		t.Fatal("ownerless empty share config should be public")
	}

	invalid := &SavedView{SharedWith: "{oops"}
	if cfg := invalid.ShareConfig(); len(cfg.RoleIDs) != 0 || len(cfg.MemberIDs) != 0 {
		t.Fatalf("invalid ShareConfig() = %#v, want empty config", cfg)
	}
}

func TestReviewToDTOClonesExecutionMetadata(t *testing.T) {
	reviewID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	taskID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	now := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)

	review := &Review{
		ID:        reviewID,
		TaskID:    taskID,
		PRURL:     "https://example.test/pr/12",
		PRNumber:  12,
		Layer:     ReviewLayerDeep,
		Status:    ReviewStatusCompleted,
		RiskLevel: ReviewRiskLevelHigh,
		Findings: []ReviewFinding{{
			Category: "security",
			Severity: "high",
			Message:  "unsafe call",
		}},
		ExecutionMetadata: &ReviewExecutionMetadata{
			ProjectID:    "project-1",
			ChangedFiles: []string{"src/main.go"},
			Dimensions:   []string{"security"},
			Results: []ReviewExecutionResult{{
				ID:     "result-1",
				Kind:   ReviewExecutionKindPlugin,
				Status: ReviewExecutionStatusCompleted,
			}},
			Decisions: []ReviewDecision{{
				Actor:     "reviewer",
				Action:    "approve",
				Comment:   "looks good",
				Timestamp: now,
			}},
		},
		Summary:        "High risk finding detected",
		Recommendation: ReviewRecommendationRequestChanges,
		CostUSD:        3.2,
		CreatedAt:      now,
		UpdatedAt:      now.Add(time.Minute),
	}

	dto := review.ToDTO()
	if dto.TaskID != taskID.String() {
		t.Fatalf("dto.TaskID = %q, want %q", dto.TaskID, taskID.String())
	}
	if dto.ExecutionMetadata == nil {
		t.Fatal("dto.ExecutionMetadata = nil, want cloned metadata")
	}
	dto.ExecutionMetadata.ChangedFiles[0] = "mutated.go"
	if review.ExecutionMetadata.ChangedFiles[0] != "src/main.go" {
		t.Fatal("expected ChangedFiles to be cloned")
	}

	if CloneReviewExecutionMetadata(nil) != nil {
		t.Fatal("CloneReviewExecutionMetadata(nil) should return nil")
	}

	nilTask := (&Review{ID: reviewID}).ToDTO()
	if nilTask.TaskID != "" {
		t.Fatalf("nilTask.TaskID = %q, want empty string", nilTask.TaskID)
	}
}

func TestMemberStatusHelpersAndToDTO(t *testing.T) {
	if got := NormalizeMemberStatus("", true); got != MemberStatusActive {
		t.Fatalf("NormalizeMemberStatus(empty, true) = %q, want %q", got, MemberStatusActive)
	}
	if got := NormalizeMemberStatus("unknown", false); got != MemberStatusInactive {
		t.Fatalf("NormalizeMemberStatus(unknown, false) = %q, want %q", got, MemberStatusInactive)
	}
	if !IsMemberStatusActive(MemberStatusActive) {
		t.Fatal("IsMemberStatusActive(active) = false, want true")
	}
	if IsMemberStatusActive(MemberStatusSuspended) {
		t.Fatal("IsMemberStatusActive(suspended) = true, want false")
	}

	userID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	member := &Member{
		ID:          uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		ProjectID:   uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		UserID:      &userID,
		Name:        "QA Bot",
		Type:        MemberTypeAgent,
		Role:        "Verifier",
		Status:      "",
		Email:       "qa@example.com",
		IMPlatform:  "feishu",
		IMUserID:    "ou_123",
		AvatarURL:   "https://example.test/avatar.png",
		AgentConfig: `{"runtime":"codex"}`,
		Skills:      []string{"verify", "report"},
		IsActive:    true,
		CreatedAt:   time.Date(2026, 3, 30, 11, 0, 0, 0, time.UTC),
	}

	dto := member.ToDTO()
	if dto.UserID == nil || *dto.UserID != userID.String() {
		t.Fatalf("dto.UserID = %#v, want %q", dto.UserID, userID.String())
	}
	if dto.Status != MemberStatusActive || !dto.IsActive {
		t.Fatalf("dto status/isActive = %q/%v, want active/true", dto.Status, dto.IsActive)
	}
	if !reflect.DeepEqual(dto.Skills, member.Skills) {
		t.Fatalf("dto.Skills = %#v, want %#v", dto.Skills, member.Skills)
	}
}

func TestWikiDTOHelpers(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	deletedAt := now.Add(time.Hour)
	parentID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	creatorID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	anchorBlockID := "block-1"

	space := (&WikiSpace{
		ID:        uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"),
		ProjectID: uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"),
		CreatedAt: now,
		DeletedAt: &deletedAt,
	}).ToDTO()
	if space.DeletedAt == nil || *space.DeletedAt != deletedAt.Format(time.RFC3339) {
		t.Fatalf("space.DeletedAt = %#v", space.DeletedAt)
	}

	page := (&WikiPage{
		ID:               uuid.MustParse("10101010-1010-1010-1010-101010101010"),
		SpaceID:          uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		ParentID:         &parentID,
		Title:            "Overview",
		Content:          "content",
		ContentText:      "plain",
		Path:             "/overview",
		SortOrder:        2,
		IsTemplate:       true,
		TemplateCategory: "ops",
		IsSystem:         true,
		IsPinned:         true,
		CreatedBy:        &creatorID,
		UpdatedBy:        &creatorID,
		CreatedAt:        now,
		UpdatedAt:        now.Add(5 * time.Minute),
		DeletedAt:        &deletedAt,
	}).ToDTO()
	if page.ParentID == nil || *page.ParentID != parentID.String() {
		t.Fatalf("page.ParentID = %#v", page.ParentID)
	}
	if page.CreatedBy == nil || *page.CreatedBy != creatorID.String() {
		t.Fatalf("page.CreatedBy = %#v", page.CreatedBy)
	}

	version := (&PageVersion{
		ID:            uuid.MustParse("12121212-1212-1212-1212-121212121212"),
		PageID:        uuid.MustParse("13131313-1313-1313-1313-131313131313"),
		VersionNumber: 3,
		Name:          "v3",
		Content:       "snapshot",
		CreatedBy:     &creatorID,
		CreatedAt:     now,
	}).ToDTO()
	if version.CreatedBy == nil || *version.CreatedBy != creatorID.String() {
		t.Fatalf("version.CreatedBy = %#v", version.CreatedBy)
	}

	comment := (&PageComment{
		ID:              uuid.MustParse("14141414-1414-1414-1414-141414141414"),
		PageID:          uuid.MustParse("15151515-1515-1515-1515-151515151515"),
		AnchorBlockID:   &anchorBlockID,
		ParentCommentID: &parentID,
		Body:            "Looks good",
		Mentions:        `["user-1"]`,
		ResolvedAt:      &deletedAt,
		CreatedBy:       &creatorID,
		CreatedAt:       now,
		UpdatedAt:       now.Add(time.Minute),
		DeletedAt:       &deletedAt,
	}).ToDTO()
	if comment.AnchorBlockID == nil || *comment.AnchorBlockID != anchorBlockID {
		t.Fatalf("comment.AnchorBlockID = %#v", comment.AnchorBlockID)
	}
	if comment.ParentCommentID == nil || *comment.ParentCommentID != parentID.String() {
		t.Fatalf("comment.ParentCommentID = %#v", comment.ParentCommentID)
	}

	favorite := (&PageFavorite{
		PageID:    uuid.MustParse("16161616-1616-1616-1616-161616161616"),
		UserID:    creatorID,
		CreatedAt: now,
	}).ToDTO()
	if favorite.PageID == "" || favorite.UserID != creatorID.String() {
		t.Fatalf("favorite = %#v", favorite)
	}

	recent := (&PageRecentAccess{
		PageID:     uuid.MustParse("17171717-1717-1717-1717-171717171717"),
		UserID:     creatorID,
		AccessedAt: now,
	}).ToDTO()
	if recent.AccessedAt != now.Format(time.RFC3339) {
		t.Fatalf("recent.AccessedAt = %q, want %q", recent.AccessedAt, now.Format(time.RFC3339))
	}

	if got := formatOptionalUUID(nil); got != nil {
		t.Fatalf("formatOptionalUUID(nil) = %#v, want nil", got)
	}
	if got := formatOptionalTime(nil); got != nil {
		t.Fatalf("formatOptionalTime(nil) = %#v, want nil", got)
	}
	if got := cloneStringPointer(nil); got != nil {
		t.Fatalf("cloneStringPointer(nil) = %#v, want nil", got)
	}
	cloned := cloneStringPointer(&anchorBlockID)
	if cloned == nil || *cloned != anchorBlockID || cloned == &anchorBlockID {
		t.Fatalf("cloneStringPointer() = %#v, want cloned value", cloned)
	}
}

func TestAgentAndQueueDTOHelpers(t *testing.T) {
	now := time.Date(2026, 3, 30, 13, 0, 0, 0, time.UTC)
	completedAt := now.Add(20 * time.Minute)
	teamID := uuid.MustParse("18181818-1818-1818-1818-181818181818")
	userRun := (&AgentRun{
		ID:              uuid.MustParse("19191919-1919-1919-1919-191919191919"),
		TaskID:          uuid.MustParse("20202020-2020-2020-2020-202020202020"),
		MemberID:        uuid.MustParse("21212121-2121-2121-2121-212121212121"),
		RoleID:          "coding-agent",
		Status:          AgentRunStatusCompleted,
		Runtime:         "codex",
		Provider:        "openai",
		Model:           "gpt-5-codex",
		InputTokens:     10,
		OutputTokens:    20,
		CacheReadTokens: 5,
		CostUsd:         1.23,
		TurnCount:       4,
		ErrorMessage:    "",
		StartedAt:       now,
		CompletedAt:     &completedAt,
		CreatedAt:       now,
		TeamID:          &teamID,
		TeamRole:        TeamRoleCoder,
	}).ToDTO()
	if userRun.CompletedAt == nil || *userRun.CompletedAt != completedAt.Format(time.RFC3339) {
		t.Fatalf("userRun.CompletedAt = %#v", userRun.CompletedAt)
	}
	if userRun.TeamID == nil || *userRun.TeamID != teamID.String() {
		t.Fatalf("userRun.TeamID = %#v", userRun.TeamID)
	}

	agentRunID := "run-1"
	queueDTO := (&AgentPoolQueueEntry{
		EntryID:    "entry-1",
		ProjectID:  "project-1",
		TaskID:     "task-1",
		MemberID:   "member-1",
		Status:     AgentPoolQueueStatusQueued,
		Reason:     "busy",
		Runtime:    "codex",
		Provider:   "openai",
		Model:      "gpt-5-codex",
		RoleID:     "reviewer",
		Priority:   PriorityHigh,
		BudgetUSD:  3.5,
		AgentRunID: &agentRunID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}).ToDTO()
	if queueDTO.Status != string(AgentPoolQueueStatusQueued) {
		t.Fatalf("queueDTO.Status = %q", queueDTO.Status)
	}
	if queueDTO.AgentRunID == nil || *queueDTO.AgentRunID != agentRunID {
		t.Fatalf("queueDTO.AgentRunID = %#v", queueDTO.AgentRunID)
	}
	if dto := (*AgentPoolQueueEntry)(nil).ToDTO(); dto != (QueueEntryDTO{}) {
		t.Fatalf("nil QueueEntryDTO = %#v, want zero value", dto)
	}

	memoryDTO := (&AgentMemory{
		ID:             uuid.MustParse("22222222-3333-4444-5555-666666666666"),
		ProjectID:      uuid.MustParse("23232323-2323-2323-2323-232323232323"),
		Scope:          MemoryScopeProject,
		RoleID:         "planner",
		Category:       MemoryCategoryProcedural,
		Key:            "deploy",
		Content:        "Use staged rollout",
		Metadata:       `{"source":"ops"}`,
		RelevanceScore: 0.8,
		AccessCount:    2,
		CreatedAt:      now,
	}).ToDTO()
	if memoryDTO.Scope != MemoryScopeProject || memoryDTO.AccessCount != 2 {
		t.Fatalf("memoryDTO = %#v", memoryDTO)
	}
}

func TestAgentTeamAutomationAndCustomFieldDTOHelpers(t *testing.T) {
	now := time.Date(2026, 3, 30, 14, 0, 0, 0, time.UTC)
	deletedAt := now.Add(time.Hour)
	plannerID := uuid.MustParse("24242424-2424-2424-2424-242424242424")
	reviewerID := uuid.MustParse("25252525-2525-2525-2525-252525252525")

	team := &AgentTeam{
		ID:             uuid.MustParse("26262626-2626-2626-2626-262626262626"),
		ProjectID:      uuid.MustParse("27272727-2727-2727-2727-272727272727"),
		TaskID:         uuid.MustParse("28282828-2828-2828-2828-282828282828"),
		Name:           "Alpha Team",
		Status:         TeamStatusExecuting,
		Strategy:       "parallel",
		PlannerRunID:   &plannerID,
		ReviewerRunID:  &reviewerID,
		TotalBudgetUsd: 10,
		TotalSpentUsd:  4.5,
		Config:         `{"runtime":"codex","provider":"openai","model":"gpt-5-codex"}`,
		ErrorMessage:   "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if selection := team.CodingAgentSelection(); selection.Runtime != "codex" || selection.Provider != "openai" {
		t.Fatalf("CodingAgentSelection() = %#v", selection)
	}
	teamDTO := team.ToDTO()
	if teamDTO.Runtime != "codex" || teamDTO.Provider != "openai" || teamDTO.Model != "gpt-5-codex" {
		t.Fatalf("teamDTO = %#v", teamDTO)
	}
	if !IsTerminalTeamStatus(TeamStatusCompleted) || IsTerminalTeamStatus(TeamStatusExecuting) {
		t.Fatal("IsTerminalTeamStatus() mismatch")
	}
	if selection := (&AgentTeam{Config: "{oops"}).CodingAgentSelection(); selection != (CodingAgentSelection{}) {
		t.Fatalf("invalid team selection = %#v, want zero value", selection)
	}

	ruleDTO := (&AutomationRule{
		ID:         uuid.MustParse("29292929-2929-2929-2929-292929292929"),
		ProjectID:  uuid.MustParse("30303030-3030-3030-3030-303030303030"),
		Name:       "Auto Review",
		Enabled:    true,
		EventType:  AutomationEventReviewCompleted,
		Conditions: "",
		Actions:    `[{"type":"notify"}]`,
		CreatedBy:  plannerID,
		CreatedAt:  now,
		UpdatedAt:  now,
		DeletedAt:  &deletedAt,
	}).ToDTO()
	if string(ruleDTO.Conditions) != "[]" || string(ruleDTO.Actions) != `[{"type":"notify"}]` {
		t.Fatalf("ruleDTO = %#v", ruleDTO)
	}
	if ruleDTO.DeletedAt == nil {
		t.Fatal("ruleDTO.DeletedAt should be populated")
	}

	logDTO := (&AutomationLog{
		ID:          uuid.MustParse("31313131-3131-3131-3131-313131313131"),
		RuleID:      uuid.MustParse("32323232-3232-3232-3232-323232323232"),
		TaskID:      &plannerID,
		EventType:   AutomationEventTaskStatusChanged,
		TriggeredAt: now,
		Status:      AutomationLogStatusSuccess,
		Detail:      "",
	}).ToDTO()
	if logDTO.TaskID == nil || *logDTO.TaskID != plannerID.String() || string(logDTO.Detail) != "{}" {
		t.Fatalf("logDTO = %#v", logDTO)
	}

	fieldDTO := (&CustomFieldDefinition{
		ID:        uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		ProjectID: uuid.MustParse("34343434-3434-3434-3434-343434343434"),
		Name:      "Severity",
		FieldType: CustomFieldTypeSelect,
		Options:   "",
		SortOrder: 3,
		Required:  true,
		CreatedAt: now,
		UpdatedAt: now,
		DeletedAt: &deletedAt,
	}).ToDTO()
	if string(fieldDTO.Options) != "[]" {
		t.Fatalf("fieldDTO.Options = %s, want []", string(fieldDTO.Options))
	}

	valueDTO := (&CustomFieldValue{
		ID:         uuid.MustParse("35353535-3535-3535-3535-353535353535"),
		TaskID:     uuid.MustParse("36363636-3636-3636-3636-363636363636"),
		FieldDefID: uuid.MustParse("37373737-3737-3737-3737-373737373737"),
		Value:      "",
		CreatedAt:  now,
		UpdatedAt:  now,
	}).ToDTO()
	if string(valueDTO.Value) != "null" {
		t.Fatalf("valueDTO.Value = %s, want null", string(valueDTO.Value))
	}

	if got := string(normalizeJSONRawMessage("  {\"a\":1}  ", []byte("[]"))); got != `{"a":1}` {
		t.Fatalf("normalizeJSONRawMessage() = %s", got)
	}
}

func TestDashboardReviewAggregationFormNotificationAndProjectDTOHelpers(t *testing.T) {
	now := time.Date(2026, 3, 30, 15, 0, 0, 0, time.UTC)
	deletedAt := now.Add(15 * time.Minute)
	userID := uuid.MustParse("38383838-3838-3838-3838-383838383838")

	widgetDTO := (&DashboardWidget{
		ID:          uuid.MustParse("39393939-3939-3939-3939-393939393939"),
		DashboardID: uuid.MustParse("40404040-4040-4040-4040-404040404040"),
		WidgetType:  DashboardWidgetBudgetConsumption,
		Config:      "",
		Position:    `{"x":1}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).ToDTO()
	if string(widgetDTO.Config) != "{}" || string(widgetDTO.Position) != `{"x":1}` {
		t.Fatalf("widgetDTO = %#v", widgetDTO)
	}

	configDTO := (&DashboardConfig{
		ID:        uuid.MustParse("41414141-4141-4141-4141-414141414141"),
		ProjectID: uuid.MustParse("42424242-4242-4242-4242-424242424242"),
		Name:      "Ops Dashboard",
		Layout:    "",
		CreatedBy: userID,
		CreatedAt: now,
		UpdatedAt: now,
		DeletedAt: &deletedAt,
	}).ToDTO([]DashboardWidgetDTO{widgetDTO})
	if len(configDTO.Widgets) != 1 || string(configDTO.Layout) != "[]" {
		t.Fatalf("configDTO = %#v", configDTO)
	}

	decision := "approve"
	comment := "ship it"
	aggregationDTO := (&ReviewAggregation{
		ID:             uuid.MustParse("43434343-4343-4343-4343-434343434343"),
		PRURL:          "https://example.test/pr/3",
		TaskID:         uuid.MustParse("44444444-4444-4444-4444-444444444444"),
		ReviewIDs:      []uuid.UUID{uuid.MustParse("45454545-4545-4545-4545-454545454545")},
		OverallRisk:    ReviewRiskLevelLow,
		Recommendation: ReviewRecommendationApprove,
		Findings:       `[]`,
		Summary:        "clear",
		Metrics:        `{"layers":3}`,
		HumanDecision:  &decision,
		HumanComment:   &comment,
		DecidedAt:      &deletedAt,
		TotalCostUsd:   2.5,
		CreatedAt:      now,
	}).ToDTO()
	if len(aggregationDTO.ReviewIDs) != 1 || aggregationDTO.DecidedAt == nil {
		t.Fatalf("aggregationDTO = %#v", aggregationDTO)
	}

	formDTO := (&FormDefinition{
		ID:             uuid.MustParse("46464646-4646-4646-4646-464646464646"),
		ProjectID:      uuid.MustParse("47474747-4747-4747-4747-474747474747"),
		Name:           "Bug Intake",
		Slug:           "bug-intake",
		Fields:         "",
		TargetStatus:   TaskStatusInbox,
		TargetAssignee: &userID,
		IsPublic:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
		DeletedAt:      &deletedAt,
	}).ToDTO()
	if formDTO.TargetAssignee == nil || string(formDTO.Fields) != "[]" {
		t.Fatalf("formDTO = %#v", formDTO)
	}
	submissionDTO := (&FormSubmission{
		ID:          uuid.MustParse("48484848-4848-4848-4848-484848484848"),
		FormID:      uuid.MustParse("49494949-4949-4949-4949-494949494949"),
		TaskID:      uuid.MustParse("50505050-5050-5050-5050-505050505050"),
		SubmittedBy: "user-1",
		SubmittedAt: now,
		IPAddress:   "127.0.0.1",
	}).ToDTO()
	if submissionDTO.IPAddress != "127.0.0.1" {
		t.Fatalf("submissionDTO = %#v", submissionDTO)
	}

	notificationDTO := (&Notification{
		ID:        uuid.MustParse("51515151-5151-5151-5151-515151515151"),
		TargetID:  userID,
		Type:      NotificationTypeTaskCreated,
		Title:     "Task created",
		Body:      "A new task is ready",
		Data:      `{"taskId":"1"}`,
		IsRead:    true,
		Channel:   NotificationChannelInApp,
		Sent:      true,
		CreatedAt: now,
	}).ToDTO()
	if notificationDTO.Channel != NotificationChannelInApp || !notificationDTO.IsRead {
		t.Fatalf("notificationDTO = %#v", notificationDTO)
	}

	project := &Project{
		ID:            uuid.MustParse("52525252-5252-5252-5252-525252525252"),
		Name:          "AgentForge",
		Slug:          "agentforge",
		Description:   "core repo",
		RepoURL:       "https://example.test/repo.git",
		DefaultBranch: "main",
		Settings:      `{"coding_agent":{"runtime":"codex","provider":"openai","model":"gpt-5-codex"},"webhook":{"url":"https://example.test/hook","secret":"top-secret","events":["task.created"],"active":true}}`,
		CreatedAt:     now,
	}
	projectDTO := project.ToDTO()
	if projectDTO.Settings.Webhook.Secret != "" {
		t.Fatalf("projectDTO.Settings.Webhook.Secret = %q, want blank", projectDTO.Settings.Webhook.Secret)
	}
	catalog := &CodingAgentCatalogDTO{DefaultRuntime: "codex"}
	withCatalog := project.ToDTOWithCatalog(catalog)
	if withCatalog.CodingAgentCatalog != catalog {
		t.Fatal("expected catalog to be attached to DTO")
	}
}

func TestSprintMilestoneAndIMFallbackHelpers(t *testing.T) {
	start := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 7, 18, 0, 0, 0, time.UTC)
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	milestoneID := uuid.MustParse("53535353-5353-5353-5353-535353535353")
	completedAt := time.Date(2026, 3, 2, 16, 0, 0, 0, time.UTC)

	sprint := &Sprint{
		ID:             uuid.MustParse("54545454-5454-5454-5454-545454545454"),
		ProjectID:      uuid.MustParse("55555555-6666-7777-8888-999999999999"),
		Name:           "Sprint 1",
		StartDate:      start,
		EndDate:        end,
		MilestoneID:    &milestoneID,
		Status:         SprintStatusActive,
		TotalBudgetUsd: 12.5,
		SpentUsd:       4.2,
		CreatedAt:      start,
	}

	sprintDTO := sprint.ToDTO()
	if sprintDTO.MilestoneID == nil || *sprintDTO.MilestoneID != milestoneID.String() {
		t.Fatalf("sprintDTO.MilestoneID = %#v", sprintDTO.MilestoneID)
	}
	if err := ValidateSprintTransition(SprintStatusPlanning, SprintStatusActive); err != nil {
		t.Fatalf("ValidateSprintTransition() error = %v", err)
	}
	if err := ValidateSprintTransition("unknown", SprintStatusActive); err == nil {
		t.Fatal("expected unknown sprint status transition to fail")
	}

	tasks := []*Task{
		{
			ID:          uuid.MustParse("56565656-5656-5656-5656-565656565656"),
			ProjectID:   sprint.ProjectID,
			Title:       "Done task",
			Status:      TaskStatusDone,
			BudgetUsd:   5,
			SpentUsd:    3.5,
			CompletedAt: &completedAt,
			UpdatedAt:   completedAt,
		},
		{
			ID:        uuid.MustParse("57575757-5757-5757-5757-575757575757"),
			ProjectID: sprint.ProjectID,
			Title:     "Open task",
			Status:    TaskStatusInProgress,
			BudgetUsd: 4,
			SpentUsd:  1.25,
			UpdatedAt: now,
		},
		nil,
	}
	metrics := BuildSprintMetricsDTO(sprint, tasks, now)
	if metrics.PlannedTasks != 3 || metrics.CompletedTasks != 1 || metrics.RemainingTasks != 2 {
		t.Fatalf("metrics counts = %#v", metrics)
	}
	if metrics.CompletionRate != 33.33 {
		t.Fatalf("metrics.CompletionRate = %v, want 33.33", metrics.CompletionRate)
	}
	if metrics.VelocityPerWeek != 1.75 {
		t.Fatalf("metrics.VelocityPerWeek = %v, want 1.75", metrics.VelocityPerWeek)
	}
	if metrics.TaskBudgetUsd != 9 || metrics.TaskSpentUsd != 4.75 {
		t.Fatalf("metrics budget/spent = %v/%v", metrics.TaskBudgetUsd, metrics.TaskSpentUsd)
	}
	if len(metrics.Burndown) != 7 || metrics.Burndown[1].CompletedTasks != 1 {
		t.Fatalf("metrics.Burndown = %#v", metrics.Burndown)
	}
	if normalizeSprintDayStart(start) != time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("normalizeSprintDayStart() mismatch")
	}
	if completed := taskCompletionTime(tasks[1]); completed != nil {
		t.Fatalf("taskCompletionTime(open task) = %#v, want nil", completed)
	}
	if rounded := roundTo2(1.236); rounded != 1.24 {
		t.Fatalf("roundTo2(1.236) = %v, want 1.24", rounded)
	}

	milestoneDate := end.AddDate(0, 0, 7)
	milestoneDeletedAt := end.AddDate(0, 0, 14)
	milestoneMetrics := BuildMilestoneMetrics(3, 2, 1)
	if milestoneMetrics.CompletionRate != 66.67 {
		t.Fatalf("milestoneMetrics.CompletionRate = %v, want 66.67", milestoneMetrics.CompletionRate)
	}
	milestoneDTO := (&Milestone{
		ID:          milestoneID,
		ProjectID:   sprint.ProjectID,
		Name:        "Launch",
		TargetDate:  &milestoneDate,
		Status:      MilestoneStatusInProgress,
		Description: "launch target",
		CreatedAt:   start,
		UpdatedAt:   now,
		DeletedAt:   &milestoneDeletedAt,
	}).ToDTO(&milestoneMetrics)
	if milestoneDTO.TargetDate == nil || *milestoneDTO.TargetDate != milestoneDate.Format(time.RFC3339) {
		t.Fatalf("milestoneDTO.TargetDate = %#v", milestoneDTO.TargetDate)
	}
	if milestoneDTO.Metrics == nil || milestoneDTO.Metrics.TotalTasks != 3 {
		t.Fatalf("milestoneDTO.Metrics = %#v", milestoneDTO.Metrics)
	}

	fallback := (&IMStructuredMessage{
		Title: "Build Ready",
		Body:  "All checks passed",
		Fields: []IMStructuredField{
			{Label: "Status", Value: "success"},
			{Value: "plain value"},
		},
		Actions: []IMStructuredAction{
			{Label: "Open", URL: "https://example.test/run/1"},
			{Label: "Retry"},
			{URL: "https://example.test/help"},
		},
	}).FallbackText()
	wantFallback := "Build Ready\nAll checks passed\nStatus: success\nplain value\nOpen: https://example.test/run/1\nRetry\nhttps://example.test/help"
	if fallback != wantFallback {
		t.Fatalf("FallbackText() = %q, want %q", fallback, wantFallback)
	}
	if got := (*IMStructuredMessage)(nil).FallbackText(); got != "" {
		t.Fatalf("nil FallbackText() = %q, want empty", got)
	}
}
