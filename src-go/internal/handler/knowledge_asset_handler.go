package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/knowledge"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

// knowledgeAssetHandlerService is the interface the handler requires.
type knowledgeAssetHandlerService interface {
	Create(ctx context.Context, pc model.PrincipalContext, a *model.KnowledgeAsset) (*model.KnowledgeAsset, error)
	Get(ctx context.Context, pc model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error)
	Update(ctx context.Context, pc model.PrincipalContext, id uuid.UUID, req model.UpdateKnowledgeAssetRequest) (*model.KnowledgeAsset, error)
	Delete(ctx context.Context, pc model.PrincipalContext, id uuid.UUID) error
	Restore(ctx context.Context, pc model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error)
	List(ctx context.Context, pc model.PrincipalContext, projectID uuid.UUID, kind *model.KnowledgeAssetKind) ([]*model.KnowledgeAsset, error)
	ListTree(ctx context.Context, pc model.PrincipalContext, spaceID uuid.UUID) ([]*model.KnowledgeAsset, error)
	Move(ctx context.Context, pc model.PrincipalContext, id uuid.UUID, req model.MoveKnowledgeAssetRequest) (*model.KnowledgeAsset, error)
	ListVersions(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID) ([]*model.AssetVersion, error)
	CreateVersion(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID, name string) (*model.AssetVersion, error)
	GetVersion(ctx context.Context, pc model.PrincipalContext, versionID uuid.UUID) (*model.AssetVersion, error)
	RestoreVersion(ctx context.Context, pc model.PrincipalContext, assetID, versionID uuid.UUID) (*model.KnowledgeAsset, *model.AssetVersion, error)
	ListComments(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID) ([]*model.AssetComment, error)
	CreateComment(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID, req model.CreateAssetCommentRequest) (*model.AssetComment, error)
	UpdateComment(ctx context.Context, pc model.PrincipalContext, assetID, commentID uuid.UUID, req model.UpdateAssetCommentRequest) (*model.AssetComment, error)
	DeleteComment(ctx context.Context, pc model.PrincipalContext, assetID, commentID uuid.UUID) error
	Search(ctx context.Context, pc model.PrincipalContext, projectID uuid.UUID, query string, kind *model.KnowledgeAssetKind, limit int) ([]*model.KnowledgeSearchResult, error)
	MaterializeAsWiki(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID, req model.MaterializeAsWikiRequest) (*model.KnowledgeAsset, error)
}

// knowledgeUploadService handles file ingest alongside asset creation.
type knowledgeUploadService interface {
	// UploadAndCreate stores the file and creates an ingested_file asset.
	UploadAndCreate(ctx context.Context, pc model.PrincipalContext, projectID uuid.UUID, fileName string, fileSize int64, mimeType string, r io.Reader) (*model.KnowledgeAsset, error)
	// Reingest replaces the file of an existing ingested_file asset.
	Reingest(ctx context.Context, pc model.PrincipalContext, assetID uuid.UUID, fileName string, fileSize int64, mimeType string, r io.Reader) (*model.KnowledgeAsset, error)
}

// KnowledgeAssetHandler handles all /knowledge/* endpoints.
type KnowledgeAssetHandler struct {
	svc    knowledgeAssetHandlerService
	upload knowledgeUploadService
}

// NewKnowledgeAssetHandler creates a new handler with the provided service.
func NewKnowledgeAssetHandler(svc *knowledge.KnowledgeAssetService) *KnowledgeAssetHandler {
	return &KnowledgeAssetHandler{svc: svc}
}

// WithUploadService attaches an upload service for multipart operations.
func (h *KnowledgeAssetHandler) WithUploadService(upload knowledgeUploadService) *KnowledgeAssetHandler {
	h.upload = upload
	return h
}

// resolvePrincipal builds a PrincipalContext from the echo.Context.
// It reads JWT claims for the user ID and the project member role if available.
func resolvePrincipal(c echo.Context) model.PrincipalContext {
	userID := currentUserID(c)
	role := "editor" // default to editor when no project role is in context
	if p := appMiddleware.GetProject(c); p != nil {
		// If the project middleware places a member role in context, use it.
		// (Actual RBAC lookup is deferred to full member-role middleware.)
		_ = p
	}
	pc := model.PrincipalContext{ProjectRole: role}
	if userID != nil {
		pc.UserID = *userID
	}
	return pc
}

// --- List assets ---

func (h *KnowledgeAssetHandler) ListAssets(c echo.Context) error {
	pc := resolvePrincipal(c)
	projectID := appMiddleware.GetProjectID(c)

	var kind *model.KnowledgeAssetKind
	if k := c.QueryParam("kind"); k != "" {
		kk := model.KnowledgeAssetKind(k)
		kind = &kk
	}

	assets, err := h.svc.List(c.Request().Context(), pc, projectID, kind)
	if err != nil {
		return knowledgeError(c, err)
	}
	dtos := make([]model.KnowledgeAssetDTO, 0, len(assets))
	for _, a := range assets {
		dtos = append(dtos, a.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

// --- Tree ---

func (h *KnowledgeAssetHandler) GetTree(c echo.Context) error {
	pc := resolvePrincipal(c)
	spaceIDStr := c.QueryParam("spaceId")
	if spaceIDStr == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "spaceId query parameter is required"})
	}
	spaceID, err := uuid.Parse(spaceIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid spaceId"})
	}
	assets, err := h.svc.ListTree(c.Request().Context(), pc, spaceID)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, buildKnowledgeTree(assets))
}

// --- Create asset (JSON) ---

func (h *KnowledgeAssetHandler) CreateAsset(c echo.Context) error {
	// Check if this is a multipart upload (ingested_file).
	if ct := c.Request().Header.Get("Content-Type"); len(ct) > 9 && ct[:9] == "multipart" {
		return h.uploadAsset(c)
	}

	pc := resolvePrincipal(c)
	projectID := appMiddleware.GetProjectID(c)

	req := new(model.CreateKnowledgeAssetRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	wikiSpaceID, err := parseOptionalUUID(req.WikiSpaceID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid wikiSpaceId"})
	}
	parentID, err := parseOptionalUUID(req.ParentID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid parentId"})
	}

	a := &model.KnowledgeAsset{
		ProjectID:        projectID,
		WikiSpaceID:      wikiSpaceID,
		ParentID:         parentID,
		Kind:             model.KnowledgeAssetKind(req.Kind),
		Title:            req.Title,
		ContentJSON:      req.ContentJSON,
		TemplateCategory: req.TemplateCategory,
	}

	created, err := h.svc.Create(c.Request().Context(), pc, a)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusCreated, created.ToDTO())
}

func (h *KnowledgeAssetHandler) uploadAsset(c echo.Context) error {
	if h.upload == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "file upload not configured"})
	}
	pc := resolvePrincipal(c)
	projectID := appMiddleware.GetProjectID(c)

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "file is required in multipart form"})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to open uploaded file"})
	}
	defer src.Close()

	a, err := h.upload.UploadAndCreate(c.Request().Context(), pc, projectID, file.Filename, file.Size, file.Header.Get("Content-Type"), src)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusCreated, a.ToDTO())
}

// --- Get single asset ---

func (h *KnowledgeAssetHandler) GetAsset(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	a, err := h.svc.Get(c.Request().Context(), pc, id)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, a.ToDTO())
}

// --- Update asset ---

func (h *KnowledgeAssetHandler) UpdateAsset(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	req := new(model.UpdateKnowledgeAssetRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	a, err := h.svc.Update(c.Request().Context(), pc, id, *req)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, a.ToDTO())
}

// --- Delete asset ---

func (h *KnowledgeAssetHandler) DeleteAsset(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	if err := h.svc.Delete(c.Request().Context(), pc, id); err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "knowledge asset deleted"})
}

// --- Restore deleted asset ---

func (h *KnowledgeAssetHandler) RestoreAsset(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	a, err := h.svc.Restore(c.Request().Context(), pc, id)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, a.ToDTO())
}

// --- Move asset ---

func (h *KnowledgeAssetHandler) MoveAsset(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	req := new(model.MoveKnowledgeAssetRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	a, err := h.svc.Move(c.Request().Context(), pc, id, *req)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, a.ToDTO())
}

// --- Reupload (replace file) ---

func (h *KnowledgeAssetHandler) ReuploadAsset(c echo.Context) error {
	if h.upload == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "file upload not configured"})
	}
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "file is required"})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to open uploaded file"})
	}
	defer src.Close()

	a, err := h.upload.Reingest(c.Request().Context(), pc, id, file.Filename, file.Size, file.Header.Get("Content-Type"), src)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, a.ToDTO())
}

// --- Materialize ingested_file as wiki_page ---

func (h *KnowledgeAssetHandler) MaterializeAsWiki(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	req := new(model.MaterializeAsWikiRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	a, err := h.svc.MaterializeAsWiki(c.Request().Context(), pc, id, *req)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusCreated, a.ToDTO())
}

// --- Versions ---

func (h *KnowledgeAssetHandler) ListVersions(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	versions, err := h.svc.ListVersions(c.Request().Context(), pc, id)
	if err != nil {
		return knowledgeError(c, err)
	}
	dtos := make([]model.AssetVersionDTO, 0, len(versions))
	for _, v := range versions {
		dtos = append(dtos, v.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *KnowledgeAssetHandler) CreateVersion(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	req := new(model.CreateAssetVersionRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	v, err := h.svc.CreateVersion(c.Request().Context(), pc, id, req.Name)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusCreated, v.ToDTO())
}

func (h *KnowledgeAssetHandler) GetVersion(c echo.Context) error {
	pc := resolvePrincipal(c)
	vid, err := uuid.Parse(c.Param("vid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid version id"})
	}
	v, err := h.svc.GetVersion(c.Request().Context(), pc, vid)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, v.ToDTO())
}

func (h *KnowledgeAssetHandler) RestoreVersion(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	vid, err := uuid.Parse(c.Param("vid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid version id"})
	}
	a, ver, err := h.svc.RestoreVersion(c.Request().Context(), pc, id, vid)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"asset":   a.ToDTO(),
		"version": ver.ToDTO(),
	})
}

// --- Comments ---

func (h *KnowledgeAssetHandler) ListComments(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	comments, err := h.svc.ListComments(c.Request().Context(), pc, id)
	if err != nil {
		return knowledgeError(c, err)
	}
	dtos := make([]model.AssetCommentDTO, 0, len(comments))
	for _, cmt := range comments {
		dtos = append(dtos, cmt.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *KnowledgeAssetHandler) CreateComment(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	req := new(model.CreateAssetCommentRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	cmt, err := h.svc.CreateComment(c.Request().Context(), pc, id, *req)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusCreated, cmt.ToDTO())
}

func (h *KnowledgeAssetHandler) UpdateComment(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	cid, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid comment id"})
	}
	req := new(model.UpdateAssetCommentRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	cmt, err := h.svc.UpdateComment(c.Request().Context(), pc, id, cid, *req)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, cmt.ToDTO())
}

func (h *KnowledgeAssetHandler) DeleteComment(c echo.Context) error {
	pc := resolvePrincipal(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid asset id"})
	}
	cid, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid comment id"})
	}
	if err := h.svc.DeleteComment(c.Request().Context(), pc, id, cid); err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "comment deleted"})
}

// --- Search ---

func (h *KnowledgeAssetHandler) Search(c echo.Context) error {
	pc := resolvePrincipal(c)
	projectID := appMiddleware.GetProjectID(c)
	query := c.QueryParam("q")
	if query == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "q query parameter is required"})
	}
	var kind *model.KnowledgeAssetKind
	if k := c.QueryParam("kind"); k != "" {
		kk := model.KnowledgeAssetKind(k)
		kind = &kk
	}
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	results, err := h.svc.Search(c.Request().Context(), pc, projectID, query, kind, limit)
	if err != nil {
		return knowledgeError(c, err)
	}
	return c.JSON(http.StatusOK, results)
}

// --- DecomposeTasks (delegate to existing doc decompose logic) ---

func (h *KnowledgeAssetHandler) DecomposeTasks(c echo.Context) error {
	// This endpoint is wired to the existing DocDecompositionHandler for now.
	return c.JSON(http.StatusNotImplemented, map[string]string{"message": "use the doc-decompose endpoint"})
}

// --- helpers ---

func knowledgeError(c echo.Context, err error) error {
	switch err {
	case knowledge.ErrAssetNotFound, knowledge.ErrCommentNotFound, knowledge.ErrVersionNotFound:
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
	case knowledge.ErrAssetForbidden:
		return c.JSON(http.StatusForbidden, model.ErrorResponse{Message: err.Error()})
	case knowledge.ErrAssetConflict:
		return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
	case knowledge.ErrCircularMove:
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	case knowledge.ErrIngestNotReady:
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	default:
		if isInvariantErr(err) {
			return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "internal server error"})
	}
}

func isInvariantErr(err error) bool {
	// check by unwrapping if it wraps ErrInvariantViolation
	type unwrapper interface{ Unwrap() []error }
	if uw, ok := err.(unwrapper); ok {
		for _, e := range uw.Unwrap() {
			if e == knowledge.ErrInvariantViolation {
				return true
			}
		}
	}
	return false
}

func buildKnowledgeTree(pages []*model.KnowledgeAsset) []model.KnowledgeAssetTreeNodeDTO {
	nodes := make(map[string]*model.KnowledgeAssetTreeNodeDTO, len(pages))
	rootIDs := make([]string, 0)
	for _, page := range pages {
		if page == nil {
			continue
		}
		dto := model.KnowledgeAssetTreeNodeDTO{
			KnowledgeAssetDTO: page.ToDTO(),
			Children:          []model.KnowledgeAssetTreeNodeDTO{},
		}
		nodes[page.ID.String()] = &dto
	}
	for _, page := range pages {
		if page == nil {
			continue
		}
		node := nodes[page.ID.String()]
		if page.ParentID == nil {
			rootIDs = append(rootIDs, page.ID.String())
			continue
		}
		parent := nodes[page.ParentID.String()]
		if parent == nil {
			rootIDs = append(rootIDs, page.ID.String())
			continue
		}
		parent.Children = append(parent.Children, *node)
	}
	roots := make([]model.KnowledgeAssetTreeNodeDTO, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		if root := nodes[rootID]; root != nil {
			roots = append(roots, rebuildKnowledgeTreeNode(*root, nodes))
		}
	}
	return roots
}

func rebuildKnowledgeTreeNode(node model.KnowledgeAssetTreeNodeDTO, nodes map[string]*model.KnowledgeAssetTreeNodeDTO) model.KnowledgeAssetTreeNodeDTO {
	if len(node.Children) == 0 {
		return node
	}
	children := make([]model.KnowledgeAssetTreeNodeDTO, 0, len(node.Children))
	for _, child := range node.Children {
		if stored := nodes[child.ID]; stored != nil {
			children = append(children, rebuildKnowledgeTreeNode(*stored, nodes))
		} else {
			children = append(children, child)
		}
	}
	node.Children = children
	return node
}
