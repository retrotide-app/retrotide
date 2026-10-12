package routes

import (
	"retrotide/internal/controllers"

	"github.com/gin-gonic/gin"
)

func PublicRoutes(ctrl *controllers.Service, engine *gin.Engine) {
	controllers.RegisterRoutes(engine.Group("/api/v1"), ctrl.GetAPIRoutes())
	controllers.RegisterRoutes(engine, ctrl.GetPageRoutes())
	engine.NoRoute(ctrl.SPA)
}
