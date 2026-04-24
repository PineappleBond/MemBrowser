package handler

import (
	"strconv"

	"github.com/PineappleBond/MemBrowser/backend/internal/model"
	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
	"github.com/gin-gonic/gin"
)

// PollUpdates: V0.1 简化 — last_seq=0 时返回所有更新 (受 limit 限制)，不特殊处理
func (h *Handler) PollUpdates(c *gin.Context) {
	lastSeq, _ := strconv.ParseInt(c.Query("last_seq"), 10, 64)
	limit := c.DefaultQuery("limit", "500")
	limitInt, _ := strconv.Atoi(limit)
	if limitInt > 1000 {
		limitInt = 1000
	}

	var updates []model.Update
	h.db.Where("seq > ?", lastSeq).Order("seq ASC").
		Limit(limitInt + 1).Find(&updates) // 多查一条判断 has_more

	hasMore := len(updates) > limitInt
	if hasMore {
		updates = updates[:limitInt]
	}

	result := ws.FillGaps(lastSeq, updates)

	// 直接返回原始 JSON (不套 pkg.OK)
	c.JSON(200, gin.H{
		"updates":  result,
		"has_more": hasMore,
		"max_seq":  h.seqMgr.Max(),
	})
}
