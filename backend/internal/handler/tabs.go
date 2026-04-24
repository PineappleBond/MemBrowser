package handler

import (
	"net/http"

	"github.com/PineappleBond/MemBrowser/backend/pkg/errors"
	"github.com/PineappleBond/MemBrowser/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

type tabOpenedReq struct {
	TabID int    `json:"tab_id" binding:"required"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

type tabClosedReq struct {
	TabID int `json:"tab_id" binding:"required"`
}

type tabActivatedReq struct {
	TabID int `json:"tab_id" binding:"required"`
}

// TabOpened 处理标签页打开事件
func (h *Handler) TabOpened(c *gin.Context) {
	var req tabOpenedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(
			errors.New(errors.ErrCodeBadRequest, "invalid request: tab_id required"),
		))
		return
	}

	h.tabMgr.OnTabOpened(req.TabID, req.URL, req.Title)
	c.JSON(http.StatusOK, response.OK(gin.H{"session_id": h.tabMgr.SessionID()}))
}

// TabClosed 处理标签页关闭事件
func (h *Handler) TabClosed(c *gin.Context) {
	var req tabClosedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(
			errors.New(errors.ErrCodeBadRequest, "invalid request: tab_id required"),
		))
		return
	}

	h.tabMgr.OnTabClosed(req.TabID)
	c.JSON(http.StatusOK, response.OK(nil))
}

// TabActivated 处理标签页激活事件
func (h *Handler) TabActivated(c *gin.Context) {
	var req tabActivatedReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(
			errors.New(errors.ErrCodeBadRequest, "invalid request: tab_id required"),
		))
		return
	}

	h.tabMgr.OnTabActivated(req.TabID)
	c.JSON(http.StatusOK, response.OK(nil))
}
