package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
)

type mockDocumentService struct {
	uploadFn func(ctx context.Context, projectID, fileName string, fileSize int64, reader io.Reader) (*model.Document, error)
	listFn   func(ctx context.Context, projectID string) ([]model.Document, error)
	getFn    func(ctx context.Context, id string) (*model.Document, error)
	deleteFn func(ctx context.Context, id string) error
}

func (m *mockDocumentService) Upload(ctx context.Context, projectID, fileName string, fileSize int64, reader io.Reader) (*model.Document, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, projectID, fileName, fileSize, reader)
	}
	return &model.Document{
		ID:        "doc-123",
		ProjectID: projectID,
		Name:      fileName,
		FileType:  ".docx",
		FileSize:  fileSize,
		Status:    model.DocumentStatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockDocumentService) List(ctx context.Context, projectID string) ([]model.Document, error) {
	if m.listFn != nil {
		return m.listFn(ctx, projectID)
	}
	return []model.Document{
		{
			ID:        "doc-123",
			ProjectID: projectID,
			Name:      "test.docx",
			FileType:  ".docx",
			Status:    model.DocumentStatusReady,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}, nil
}

func (m *mockDocumentService) Get(ctx context.Context, id string) (*model.Document, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return &model.Document{
		ID:        id,
		ProjectID: "proj-1",
		Name:      "test.docx",
		FileType:  ".docx",
		Status:    model.DocumentStatusReady,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockDocumentService) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func TestDocumentHandler_Upload(t *testing.T) {
	e := echo.New()
	svc := &mockDocumentService{}
	h := NewDocumentHandler(svc)

	// Create multipart form.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("file", "test.docx")
	if err != nil {
		t.Fatal(err)
	}
	part.Write([]byte("fake docx content"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues("proj-1")

	if err := h.Upload(c); err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("Upload() status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestDocumentHandler_List(t *testing.T) {
	e := echo.New()
	svc := &mockDocumentService{}
	h := NewDocumentHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues("proj-1")

	if err := h.List(c); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("List() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDocumentHandler_Get(t *testing.T) {
	e := echo.New()
	svc := &mockDocumentService{}
	h := NewDocumentHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid", "did")
	c.SetParamValues("proj-1", "doc-123")

	if err := h.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Get() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDocumentHandler_Delete(t *testing.T) {
	e := echo.New()
	svc := &mockDocumentService{}
	h := NewDocumentHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid", "did")
	c.SetParamValues("proj-1", "doc-123")

	if err := h.Delete(c); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("Delete() status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestDocumentHandler_NilService(t *testing.T) {
	e := echo.New()
	h := NewDocumentHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues("proj-1")

	_ = h.List(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("List() with nil service status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
