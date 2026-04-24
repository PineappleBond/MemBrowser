package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
	"github.com/PineappleBond/MemBrowser/backend/pkg/errors"
	"github.com/PineappleBond/MemBrowser/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

type TeachRequest struct {
	PageURL        string `json:"page_url" binding:"required"`
	PageTitle      string `json:"page_title"`
	ActionType     string `json:"action_type" binding:"required"`
	ActionTarget   string `json:"action_target"`
	ActionSelector string `json:"action_selector"`
	ActionValue    string `json:"action_value"`
	Screenshot     string `json:"screenshot"` // base64 编码
	DOMSnapshot    string `json:"dom_snapshot"`
}

// marshal 将任意值序列化为 json.RawMessage，用于 ws.Update.Payload
func marshalUpdatePayload(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func (h *Handler) UploadTeachData(c *gin.Context) {
	var req TeachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(
			errors.New(errors.ErrCodeBadRequest, err.Error()),
		))
		return
	}

	var screenshotPath, domPath string

	// 保存截图
	if req.Screenshot != "" {
		data, err := base64.StdEncoding.DecodeString(req.Screenshot)
		if err == nil {
			screenshotPath, _ = h.memStore.SaveScreenshot(data)
		}
	}

	// 保存 DOM 快照
	if req.DOMSnapshot != "" {
		domPath, _ = h.memStore.SaveDOMSnapshot(req.DOMSnapshot)
	}

	// 构造 Memory
	pageURLPattern := service.NormalizeURL(req.PageURL)
	mem := &model.Memory{
		SessionID:       h.wsMgr.SessionID(),
		PageURL:         req.PageURL,
		PageURLPattern:  pageURLPattern,
		PageTitle:       req.PageTitle,
		ActionType:      req.ActionType,
		ActionTarget:    req.ActionTarget,
		ActionSelector:  req.ActionSelector,
		ActionValue:     req.ActionValue,
		Result:          "success",
		Source:          "human",
		DOMSnapshotPath: domPath,
		ScreenshotPath:  screenshotPath,
	}

	if err := h.memStore.Save(mem); err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(
			errors.New(errors.ErrCodeInternal, err.Error()),
		))
		return
	}

	// 推送 update
	h.wsMgr.PushUpdate(ws.Update{
		Type: ws.UpdateTaskStep,
		Payload: marshalUpdatePayload(map[string]any{
			"content": "人类示教数据已保存",
		}),
	})

	c.JSON(http.StatusOK, response.OK(gin.H{"id": mem.ID}))
}
