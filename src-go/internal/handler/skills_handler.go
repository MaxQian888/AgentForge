package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"

	skillspkg "github.com/agentforge/server/internal/skills"
	"github.com/labstack/echo/v4"
)

type skillsService interface {
	List(opts skillspkg.ListOptions) ([]skillspkg.InventoryItem, error)
	Get(id string) (*skillspkg.InventoryItem, error)
	Verify(opts skillspkg.VerifyOptions) (*skillspkg.VerifyResult, error)
	SyncMirrors(opts skillspkg.SyncMirrorsOptions) (*skillspkg.SyncMirrorsResult, error)
}

type SkillsHandler struct {
	service skillsService
}

func NewSkillsHandler(service skillsService) *SkillsHandler {
	return &SkillsHandler{service: service}
}

type listSkillsResponse struct {
	Items []skillspkg.InventoryItem `json:"items"`
}

type verifySkillsRequest struct {
	Families []skillspkg.Family `json:"families"`
}

type syncMirrorsRequest struct {
	SkillIDs []string `json:"skillIds"`
}

func (h *SkillsHandler) List(c echo.Context) error {
	items, err := h.service.List(skillspkg.ListOptions{})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "failed to load skills"})
	}
	return c.JSON(http.StatusOK, listSkillsResponse{Items: items})
}

func (h *SkillsHandler) Get(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "missing skill id"})
	}

	item, err := h.service.Get(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "not exist") {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "skill not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "failed to load skill"})
	}
	return c.JSON(http.StatusOK, item)
}

func (h *SkillsHandler) Verify(c echo.Context) error {
	req := verifySkillsRequest{}
	if c.Request().ContentLength > 0 {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "invalid request"})
		}
	}

	result, err := h.service.Verify(skillspkg.VerifyOptions{Families: req.Families})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "failed to verify skills"})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *SkillsHandler) SyncMirrors(c echo.Context) error {
	req := syncMirrorsRequest{}
	if c.Request().ContentLength > 0 {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"message": "invalid request"})
		}
	}

	result, err := h.service.SyncMirrors(skillspkg.SyncMirrorsOptions{SkillIDs: req.SkillIDs})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "failed to sync mirrors"})
	}
	return c.JSON(http.StatusOK, result)
}
