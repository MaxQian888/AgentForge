package service

import (
	"archive/zip"
	"fmt"
	"path"
	"strings"

	"github.com/agentforge/marketplace/internal/model"
)

// ArtifactValidationError reports a stable operator-facing artifact contract failure.
type ArtifactValidationError struct {
	Message string
}

func (e *ArtifactValidationError) Error() string {
	if e == nil {
		return "invalid marketplace artifact"
	}
	return e.Message
}

func validateMarketplaceArtifact(itemType string, artifactPath string) error {
	requiredFile := requiredMarketplaceArtifactRootFile(itemType)
	if requiredFile == "" {
		return &ArtifactValidationError{
			Message: fmt.Sprintf("invalid artifact: unsupported item type %q", itemType),
		}
	}

	reader, err := zip.OpenReader(artifactPath)
	if err != nil {
		return &ArtifactValidationError{
			Message: fmt.Sprintf("invalid artifact: expected zip package: %v", err),
		}
	}
	defer reader.Close()

	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		safeName, safeErr := sanitizeArtifactZipPath(file.Name)
		if safeErr != nil {
			return safeErr
		}
		if safeName == "" || file.FileInfo().IsDir() {
			continue
		}
		names = append(names, safeName)
	}

	if len(names) == 0 {
		return &ArtifactValidationError{Message: "invalid artifact: archive is empty"}
	}
	if !zipPackageContainsRequiredRoot(names, requiredFile) {
		return &ArtifactValidationError{
			Message: fmt.Sprintf(
				"invalid %s artifact: %s is required at the package root",
				itemType,
				requiredFile,
			),
		}
	}

	return nil
}

func requiredMarketplaceArtifactRootFile(itemType string) string {
	switch itemType {
	case model.ItemTypePlugin:
		return "manifest.yaml"
	case model.ItemTypeRole:
		return "role.yaml"
	case model.ItemTypeSkill:
		return "SKILL.md"
	case model.ItemTypeWorkflowTemplate:
		return "workflow.json"
	default:
		return ""
	}
}

func sanitizeArtifactZipPath(name string) (string, error) {
	normalized := path.Clean(strings.TrimSpace(strings.ReplaceAll(name, "\\", "/")))
	switch {
	case normalized == ".":
		return "", nil
	case strings.HasPrefix(normalized, "../"), strings.Contains(normalized, "/../"), strings.HasPrefix(normalized, "/"):
		return "", &ArtifactValidationError{Message: "invalid artifact: archive contains unsafe paths"}
	default:
		return normalized, nil
	}
}

func zipPackageContainsRequiredRoot(names []string, required string) bool {
	root := detectArtifactZipRoot(names)
	requiredPath := required
	if root != "" {
		requiredPath = root + "/" + required
	}
	for _, name := range names {
		if name == requiredPath {
			return true
		}
	}
	return false
}

func detectArtifactZipRoot(names []string) string {
	root := ""
	for _, name := range names {
		parts := strings.Split(name, "/")
		if len(parts) <= 1 {
			return ""
		}
		if root == "" {
			root = parts[0]
			continue
		}
		if root != parts[0] {
			return ""
		}
	}
	return root
}
