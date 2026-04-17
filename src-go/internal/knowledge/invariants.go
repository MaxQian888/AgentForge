// Package knowledge provides the unified KnowledgeAsset domain including
// repositories, service, ingest worker, and search.
package knowledge

import (
	"errors"

	"github.com/react-go-quick-starter/server/internal/model"
)

var (
	// ErrInvariantViolation is the base sentinel for all kind-specific invariant failures.
	ErrInvariantViolation = errors.New("knowledge asset invariant violation")

	ErrAssetNotFound      = errors.New("knowledge asset not found")
	ErrAssetConflict      = errors.New("knowledge asset version conflict")
	ErrAssetForbidden     = errors.New("knowledge asset operation forbidden")
	ErrCommentNotFound    = errors.New("asset comment not found")
	ErrVersionNotFound    = errors.New("asset version not found")
	ErrCircularMove       = errors.New("cannot move asset into its own descendant")
	ErrUnsupportedKind    = errors.New("unsupported knowledge asset kind")
	ErrIngestNotReady     = errors.New("asset ingest not complete")
)

// ValidateKnowledgeAsset enforces repository invariants in Go (not SQL triggers).
// It is called before any Create or Update operation.
func ValidateKnowledgeAsset(a *model.KnowledgeAsset) error {
	if a == nil {
		return errors.Join(ErrInvariantViolation, errors.New("asset is nil"))
	}
	switch a.Kind {
	case model.KindWikiPage:
		return validateWikiPage(a)
	case model.KindIngestedFile:
		return validateIngestedFile(a)
	case model.KindTemplate:
		return validateTemplate(a)
	default:
		return errors.Join(ErrInvariantViolation, ErrUnsupportedKind)
	}
}

func validateWikiPage(a *model.KnowledgeAsset) error {
	if a.WikiSpaceID == nil {
		return errors.Join(ErrInvariantViolation, errors.New("wiki_page: wiki_space_id must not be null"))
	}
	if a.ContentJSON == "" {
		return errors.Join(ErrInvariantViolation, errors.New("wiki_page: content_json must not be empty"))
	}
	if a.FileRef != "" {
		return errors.Join(ErrInvariantViolation, errors.New("wiki_page: file_ref must be null"))
	}
	return nil
}

func validateIngestedFile(a *model.KnowledgeAsset) error {
	if a.FileRef == "" {
		return errors.Join(ErrInvariantViolation, errors.New("ingested_file: file_ref must not be empty"))
	}
	if a.ParentID != nil {
		return errors.Join(ErrInvariantViolation, errors.New("ingested_file: parent_id must be null"))
	}
	if a.WikiSpaceID != nil {
		return errors.Join(ErrInvariantViolation, errors.New("ingested_file: wiki_space_id must be null"))
	}
	if a.ContentJSON != "" {
		return errors.Join(ErrInvariantViolation, errors.New("ingested_file: content_json must be null"))
	}
	return nil
}

func validateTemplate(a *model.KnowledgeAsset) error {
	if a.WikiSpaceID == nil {
		return errors.Join(ErrInvariantViolation, errors.New("template: wiki_space_id must not be null"))
	}
	if a.ContentJSON == "" {
		return errors.Join(ErrInvariantViolation, errors.New("template: content_json must not be empty"))
	}
	if a.ParentID != nil {
		return errors.Join(ErrInvariantViolation, errors.New("template: parent_id must be null"))
	}
	if a.TemplateCategory == "" {
		return errors.Join(ErrInvariantViolation, errors.New("template: template_category must not be empty"))
	}
	return nil
}
