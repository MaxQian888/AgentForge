package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/google/uuid"
)

func TestItemTypeConstants(t *testing.T) {
	if model.ItemTypePlugin != "plugin" {
		t.Errorf("ItemTypePlugin: expected %q, got %q", "plugin", model.ItemTypePlugin)
	}
	if model.ItemTypeSkill != "skill" {
		t.Errorf("ItemTypeSkill: expected %q, got %q", "skill", model.ItemTypeSkill)
	}
	if model.ItemTypeRole != "role" {
		t.Errorf("ItemTypeRole: expected %q, got %q", "role", model.ItemTypeRole)
	}
}

func TestDefaultPageSize(t *testing.T) {
	if model.DefaultPageSize != 20 {
		t.Errorf("DefaultPageSize: expected 20, got %d", model.DefaultPageSize)
	}
}

func TestMaxPageSize(t *testing.T) {
	if model.MaxPageSize != 100 {
		t.Errorf("MaxPageSize: expected 100, got %d", model.MaxPageSize)
	}
}

func TestMarketplaceItemTableName(t *testing.T) {
	item := model.MarketplaceItem{}
	if item.TableName() != "marketplace_items" {
		t.Errorf("unexpected table name: %q", item.TableName())
	}
}

func TestMarketplaceItemVersionTableName(t *testing.T) {
	v := model.MarketplaceItemVersion{}
	if v.TableName() != "marketplace_item_versions" {
		t.Errorf("unexpected table name: %q", v.TableName())
	}
}

func TestMarketplaceReviewTableName(t *testing.T) {
	r := model.MarketplaceReview{}
	if r.TableName() != "marketplace_reviews" {
		t.Errorf("unexpected table name: %q", r.TableName())
	}
}

func TestMarketplaceItemJSONRoundtrip(t *testing.T) {
	id := uuid.New()
	authorID := uuid.New()
	tags := model.StringArray{"go", "api"}
	meta := json.RawMessage(`{"foo":"bar"}`)

	original := model.MarketplaceItem{
		ID:            id,
		Type:          model.ItemTypePlugin,
		Slug:          "my-plugin",
		Name:          "My Plugin",
		AuthorID:      authorID,
		AuthorName:    "Alice",
		Description:   "A plugin for testing",
		Category:      "testing",
		Tags:          tags,
		License:       "MIT",
		ExtraMetadata: meta,
		DownloadCount: 42,
		AvgRating:     4.5,
		RatingCount:   10,
		IsVerified:    true,
		IsFeatured:    false,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
		UpdatedAt:     time.Now().UTC().Truncate(time.Second),
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded model.MarketplaceItem
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: %v vs %v", decoded.ID, original.ID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: %q vs %q", decoded.Type, original.Type)
	}
	if decoded.Slug != original.Slug {
		t.Errorf("Slug mismatch: %q vs %q", decoded.Slug, original.Slug)
	}
	if decoded.DownloadCount != original.DownloadCount {
		t.Errorf("DownloadCount mismatch: %d vs %d", decoded.DownloadCount, original.DownloadCount)
	}
	if decoded.IsVerified != original.IsVerified {
		t.Errorf("IsVerified mismatch: %v vs %v", decoded.IsVerified, original.IsVerified)
	}
}

func TestItemListResponseFields(t *testing.T) {
	resp := model.ItemListResponse{
		Items:    []model.MarketplaceItem{},
		Total:    99,
		Page:     3,
		PageSize: 20,
	}
	if resp.Total != 99 {
		t.Errorf("Total: expected 99, got %d", resp.Total)
	}
	if resp.Page != 3 {
		t.Errorf("Page: expected 3, got %d", resp.Page)
	}
	if resp.PageSize != 20 {
		t.Errorf("PageSize: expected 20, got %d", resp.PageSize)
	}
}

func TestCreateItemRequestFields(t *testing.T) {
	req := model.CreateItemRequest{
		Type:        model.ItemTypeSkill,
		Slug:        "my-skill",
		Name:        "My Skill",
		Description: "Does stuff",
		Category:    "util",
		Tags:        []string{"a", "b"},
		License:     "Apache-2.0",
	}

	if req.Type != "skill" {
		t.Errorf("Type: expected 'skill', got %q", req.Type)
	}
	if len(req.Tags) != 2 {
		t.Errorf("Tags length: expected 2, got %d", len(req.Tags))
	}
}
