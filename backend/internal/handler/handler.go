package handler

import (
	"log/slog"

	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/PineappleBond/MemBrowser/backend/internal/ws"
	"github.com/PineappleBond/MemBrowser/backend/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	taskSvc  *service.TaskService
	wsMgr    *ws.Manager
	tabMgr   *service.TabManager
	waiter   *ws.ResponseWaiter
	memStore *service.MemoryStore
	db       *gorm.DB
	seqMgr   *ws.SeqManager
	log      *slog.Logger
}

func NewHandler(taskSvc *service.TaskService, wsMgr *ws.Manager, tabMgr *service.TabManager, waiter *ws.ResponseWaiter, memStore *service.MemoryStore, db *gorm.DB, seqMgr *ws.SeqManager, log *slog.Logger) *Handler {
	return &Handler{taskSvc: taskSvc, wsMgr: wsMgr, tabMgr: tabMgr, waiter: waiter, memStore: memStore, db: db, seqMgr: seqMgr, log: log}
}

func (h *Handler) Health(c *gin.Context) {
	c.JSON(200, response.OK(gin.H{"status": "ok"}))
}

func (h *Handler) ServeWS(c *gin.Context) {
	h.wsMgr.ServeHTTP(c.Writer, c.Request)
}
