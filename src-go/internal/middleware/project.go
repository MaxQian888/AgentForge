package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

const ProjectContextKey = "project"
const ProjectIDContextKey = "project_id"

func ProjectMiddleware(projectRepo *repository.ProjectRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			pid := c.Param("pid")
			if pid == "" {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "project ID required"})
			}
			id, err := uuid.Parse(pid)
			if err != nil {
				return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid project ID"})
			}
			project, err := projectRepo.GetByID(c.Request().Context(), id)
			if err != nil {
				return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "project not found"})
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
