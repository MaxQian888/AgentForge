package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/labstack/echo/v4"
)

// DocumentRuntimeService defines the service interface for document operations.
type DocumentRuntimeService interface {
	Upload(ctx context.Context, projectID, fileName string, fileSize int64, reader io.Reader) (*model.Document, error)
	List(ctx context.Context, projectID string) ([]model.Document, error)
	Get(ctx context.Context, id string) (*model.Document, error)
	Delete(ctx context.Context, id string) error
}

// DocumentHandler handles HTTP requests for document operations.
type DocumentHandler struct {
	svc DocumentRuntimeService
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(svc DocumentRuntimeService) *DocumentHandler {
	return &DocumentHandler{svc: svc}
}

// Upload handles multipart file upload for document parsing.
func (h *DocumentHandler) Upload(c echo.Context) error {
	if h.svc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDocumentServiceUnavailable)
	}

	projectID := c.Param("pid")
	if projectID == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "file is required in multipart form"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to open uploaded file"})
	}
	defer src.Close()

	doc, err := h.svc.Upload(c.Request().Context(), projectID, file.Filename, file.Size, src)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}

	return c.JSON(http.StatusCreated, doc.ToDTO())
}

// List returns all documents for a project.
func (h *DocumentHandler) List(c echo.Context) error {
	if h.svc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDocumentServiceUnavailable)
	}

	projectID := c.Param("pid")
	if projectID == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}

	docs, err := h.svc.List(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListDocuments)
	}

	dtos := make([]model.DocumentDTO, len(docs))
	for i := range docs {
		dtos[i] = docs[i].ToDTO()
	}
	return c.JSON(http.StatusOK, dtos)
}

// Get returns a single document by ID.
func (h *DocumentHandler) Get(c echo.Context) error {
	if h.svc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDocumentServiceUnavailable)
	}

	id := c.Param("did")
	if id == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "document id is required"})
	}

	doc, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgDocumentNotFound)
	}
	return c.JSON(http.StatusOK, doc.ToDTO())
}

// Delete removes a document and returns 204 No Content.
func (h *DocumentHandler) Delete(c echo.Context) error {
	if h.svc == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgDocumentServiceUnavailable)
	}

	id := c.Param("did")
	if id == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "document id is required"})
	}

	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteDocument)
	}
	return c.NoContent(http.StatusNoContent)
}
