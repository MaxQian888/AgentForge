package template_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/workflow/template"
)

type fakeResolver struct {
	secretsByName map[string]string
}

func (f *fakeResolver) Resolve(_ context.Context, _ uuid.UUID, name string) (string, error) {
	v, ok := f.secretsByName[name]
	if !ok {
		return "", errors.New("secret:not_found")
	}
	return v, nil
}

func TestRender_AllowedFieldSubstitutesSecret(t *testing.T) {
	r := template.NewSecretResolver(&fakeResolver{secretsByName: map[string]string{"TOKEN": "ghp_xyz"}})
	out, err := r.Render(context.Background(), uuid.New(), template.FieldHTTPHeaders, "Bearer {{secrets.TOKEN}}", nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "Bearer ghp_xyz" {
		t.Errorf("expected substitution, got %q", out)
	}
}

func TestRender_DisallowedFieldRejects(t *testing.T) {
	r := template.NewSecretResolver(&fakeResolver{secretsByName: map[string]string{"TOKEN": "ghp"}})
	_, err := r.Render(context.Background(), uuid.New(), template.FieldGeneric, "Bearer {{secrets.TOKEN}}", nil)
	if !errors.Is(err, template.ErrSecretFieldNotAllowed) {
		t.Errorf("expected ErrSecretFieldNotAllowed, got %v", err)
	}
}

func TestRender_DataStoreReferenceStillResolvedNormally(t *testing.T) {
	r := template.NewSecretResolver(nil)
	ds := map[string]any{"node1": map[string]any{"value": "ok"}}
	out, err := r.Render(context.Background(), uuid.New(), template.FieldHTTPBody, "x={{node1.value}}", ds)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out != "x=ok" {
		t.Errorf("expected dataStore substitution, got %q", out)
	}
}

func TestRender_RejectsSystemMetadataReferenceFromAuthorCode(t *testing.T) {
	r := template.NewSecretResolver(nil)
	ds := map[string]any{"system_metadata": map[string]any{"reply_target": "x"}}
	_, err := r.Render(context.Background(), uuid.New(), template.FieldHTTPBody, "x={{system_metadata.reply_target}}", ds)
	if !errors.Is(err, template.ErrSystemMetadataNotAllowed) {
		t.Errorf("expected ErrSystemMetadataNotAllowed, got %v", err)
	}
}

func TestValidateConfig_RejectsSecretInDisallowedField(t *testing.T) {
	// Save-time defense in depth.
	err := template.ValidateNoSecretReferences(template.FieldGeneric, "{{secrets.X}}")
	if !errors.Is(err, template.ErrSecretFieldNotAllowed) {
		t.Errorf("expected reject at save time, got %v", err)
	}
}

func TestValidateConfig_AllowsSecretInHTTPHeaders(t *testing.T) {
	if err := template.ValidateNoSecretReferences(template.FieldHTTPHeaders, "Bearer {{secrets.X}}"); err != nil {
		t.Errorf("expected accept at save time, got %v", err)
	}
}
