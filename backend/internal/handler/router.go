package handler

import (
	"strings"

	"github.com/PineappleBond/MemBrowser/backend/internal/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func NewRouter(h *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.ErrorHandler())

	// CORS
	r.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return strings.HasPrefix(origin, "chrome-extension://") ||
				strings.HasPrefix(origin, "http://localhost")
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.GET("/health", h.Health)

	v1 := r.Group("/api/v1")
	{
		v1.POST("/tasks", h.StartTask)
		v1.POST("/tasks/:id/stop", h.StopTask)
		v1.POST("/page/state", h.UploadPageState)
		v1.POST("/action/result", h.UploadActionResult)
		v1.POST("/teach", h.UploadTeachData)
		v1.POST("/tabs/opened", h.TabOpened)
		v1.POST("/tabs/closed", h.TabClosed)
		v1.POST("/tabs/activated", h.TabActivated)
		v1.GET("/updates", h.PollUpdates)
	}

	r.GET("/ws", h.ServeWS)
	return r
}
