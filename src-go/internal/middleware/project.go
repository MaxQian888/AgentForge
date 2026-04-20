package middleware

import (
	"net/http"

	appI18n "github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const ProjectContextKey = "project"
const ProjectIDContextKey = "project_id"

func ProjectMiddleware(projectRepo *repository.ProjectRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			pid := c.Param("pid")
			if pid == "" {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgProjectIDRequired)})
			}
			id, err := uuid.Parse(pid)
			if err != nil {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgInvalidProjectID)})
			}
			project, err := projectRepo.GetByID(c.Request().Context(), id)
			if err != nil {
				return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: appI18n.Localize(GetLocalizer(c), appI18n.MsgProjectNotFound)})
			}
			c.Set(ProjectContextKey, project)
			c.Set(ProjectIDContextKey, project.ID)
			return next(c)
		}
	}
}

func GetProject(c echo.Context) *model.Project {
	p, _ := c.Get(ProjectContextKey).(*model.Project)
	return p
}

func GetProjectID(c echo.Context) uuid.UUID {
	id, _ := c.Get(ProjectIDContextKey).(uuid.UUID)
	return id
}
