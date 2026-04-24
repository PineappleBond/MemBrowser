package handler

import (
	"encoding/json"
	"net/http"

	"github.com/PineappleBond/MemBrowser/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) StartTask(c *gin.Context) {
	var req struct {
		Task  string `json:"task" binding:"required"`
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Error: "invalid request", Details: err.Error()})
		return
	}

	taskID := uuid.New().String()
	h.taskSvc.StartTask(taskID, req.Task)

	h.log.Info("task started", "task_id", taskID, "model", req.Model)
	c.JSON(http.StatusOK, response.OK(gin.H{"task_id": taskID}))
}

func (h *Handler) StopTask(c *gin.Context) {
	taskID := c.Param("id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Error: "task id required"})
		return
	}
	h.taskSvc.StopTask(taskID)
	h.log.Info("task stopped", "task_id", taskID)
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) UploadPageState(c *gin.Context) {
	var req struct {
		RequestID     string `json:"request_id" binding:"required"`
		URL           string `json:"url"`
		Title         string `json:"title"`
		Interactables any    `json:"interactables"`
		Headings      any    `json:"headings"`
		DOM           any    `json:"dom"`
		Screenshot    string `json:"screenshot,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Error: "invalid request", Details: err.Error()})
		return
	}

	h.log.Info("page state received", "request_id", req.RequestID, "url", req.URL)

	// 兼容新旧格式: 新格式用 interactables+headings，旧格式用 dom
	domSummary := req.Interactables
	if domSummary == nil {
		domSummary = req.DOM
	}

	data, _ := json.Marshal(map[string]any{
		"url":         req.URL,
		"title":       req.Title,
		"dom_summary": domSummary,
		"headings":    req.Headings,
		"screenshot":  req.Screenshot,
	})
	h.waiter.Resolve(req.RequestID, data)
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h *Handler) UploadActionResult(c *gin.Context) {
	var req struct {
		RequestID string `json:"request_id" binding:"required"`
		Success   bool   `json:"success"`
		Message   string `json:"message,omitempty"`
		Error     string `json:"error,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Error: "invalid request", Details: err.Error()})
		return
	}

	h.log.Info("action result received", "request_id", req.RequestID, "success", req.Success)

	data, _ := json.Marshal(map[string]any{
		"success": req.Success,
		"message": req.Message,
	})
	h.waiter.Resolve(req.RequestID, data)
	c.JSON(http.StatusOK, response.OK(nil))
}
