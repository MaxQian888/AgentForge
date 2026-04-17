package knowledge_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/knowledge"
	"github.com/react-go-quick-starter/server/internal/model"
)

func ptr[T any](v T) *T { return &v }

func baseWikiPage() *model.KnowledgeAsset {
	spaceID := uuid.New()
	return &model.KnowledgeAsset{
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: `[{"type":"paragraph"}]`,
	}
}

func baseIngestedFile() *model.KnowledgeAsset {
	return &model.KnowledgeAsset{
		Kind:    model.KindIngestedFile,
		FileRef: "project/some-file.pdf",
	}
}

func baseTemplate() *model.KnowledgeAsset {
	spaceID := uuid.New()
	return &model.KnowledgeAsset{
		Kind:             model.KindTemplate,
		WikiSpaceID:      &spaceID,
		ContentJSON:      `[{"type":"paragraph"}]`,
		TemplateCategory: "meeting",
	}
}

func TestValidateKnowledgeAsset_WikiPage_Valid(t *testing.T) {
	if err := knowledge.ValidateKnowledgeAsset(baseWikiPage()); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateKnowledgeAsset_WikiPage_MissingSpace(t *testing.T) {
	a := baseWikiPage()
	a.WikiSpaceID = nil
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error for missing wiki_space_id")
	}
	if !errors.Is(err, knowledge.ErrInvariantViolation) {
		t.Fatalf("expected ErrInvariantViolation, got: %v", err)
	}
}

func TestValidateKnowledgeAsset_WikiPage_MissingContent(t *testing.T) {
	a := baseWikiPage()
	a.ContentJSON = ""
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error for empty content_json")
	}
}

func TestValidateKnowledgeAsset_WikiPage_HasFileRef(t *testing.T) {
	a := baseWikiPage()
	a.FileRef = "some/file.pdf"
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error when wiki_page has file_ref")
	}
}

func TestValidateKnowledgeAsset_IngestedFile_Valid(t *testing.T) {
	if err := knowledge.ValidateKnowledgeAsset(baseIngestedFile()); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateKnowledgeAsset_IngestedFile_MissingFileRef(t *testing.T) {
	a := baseIngestedFile()
	a.FileRef = ""
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error for missing file_ref")
	}
}

func TestValidateKnowledgeAsset_IngestedFile_HasParent(t *testing.T) {
	a := baseIngestedFile()
	parentID := uuid.New()
	a.ParentID = &parentID
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error when ingested_file has parent_id")
	}
}

func TestValidateKnowledgeAsset_IngestedFile_HasSpace(t *testing.T) {
	a := baseIngestedFile()
	spaceID := uuid.New()
	a.WikiSpaceID = &spaceID
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error when ingested_file has wiki_space_id")
	}
}

func TestValidateKnowledgeAsset_IngestedFile_HasContentJSON(t *testing.T) {
	a := baseIngestedFile()
	a.ContentJSON = `[{"type":"paragraph"}]`
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error when ingested_file has content_json")
	}
}

func TestValidateKnowledgeAsset_Template_Valid(t *testing.T) {
	if err := knowledge.ValidateKnowledgeAsset(baseTemplate()); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateKnowledgeAsset_Template_MissingSpace(t *testing.T) {
	a := baseTemplate()
	a.WikiSpaceID = nil
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error for missing wiki_space_id")
	}
}

func TestValidateKnowledgeAsset_Template_MissingCategory(t *testing.T) {
	a := baseTemplate()
	a.TemplateCategory = ""
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error for missing template_category")
	}
}

func TestValidateKnowledgeAsset_Template_HasParent(t *testing.T) {
	a := baseTemplate()
	parentID := uuid.New()
	a.ParentID = &parentID
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error when template has parent_id")
	}
}

func TestValidateKnowledgeAsset_Nil(t *testing.T) {
	err := knowledge.ValidateKnowledgeAsset(nil)
	if err == nil {
		t.Fatal("expected error for nil asset")
	}
}

func TestValidateKnowledgeAsset_UnknownKind(t *testing.T) {
	a := &model.KnowledgeAsset{Kind: "unknown_kind"}
	err := knowledge.ValidateKnowledgeAsset(a)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !errors.Is(err, knowledge.ErrUnsupportedKind) {
		t.Fatalf("expected ErrUnsupportedKind wrapped, got: %v", err)
	}
}
