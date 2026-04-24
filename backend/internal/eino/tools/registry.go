package tools

import (
	"log/slog"

	"github.com/PineappleBond/MemBrowser/backend/internal/service"
	"github.com/cloudwego/eino/components/tool"
)

type Registry struct {
	tools []tool.BaseTool
}

type RegistryDeps struct {
	WSManager WSManager
	Waiter    Waiter
	TabMgr    *service.TabManager
	MemStore  *service.MemoryStore
	SessionID string
	Log       *slog.Logger
}

func NewRegistry(deps RegistryDeps) *Registry {
	r := &Registry{}
	r.buildTools(deps)
	return r
}

func (r *Registry) GetTools() []tool.BaseTool {
	return r.tools
}

func (r *Registry) buildTools(deps RegistryDeps) {
	askHuman, err := NewAskHumanTool()
	if err != nil {
		panic("failed to create ask_human tool: " + err.Error())
	}

	getPageState, err := NewGetPageStateTool(deps.WSManager, deps.Waiter)
	if err != nil {
		panic("failed to create get_page_state tool: " + err.Error())
	}

	executeAction, err := NewExecuteActionTool(deps.WSManager, deps.Waiter)
	if err != nil {
		panic("failed to create execute_action tool: " + err.Error())
	}

	searchMemory, err := NewSearchMemoryTool(deps.MemStore)
	if err != nil {
		panic("failed to create search_memory tool: " + err.Error())
	}

	saveMemory, err := NewSaveMemoryTool(deps.MemStore, deps.SessionID)
	if err != nil {
		panic("failed to create save_memory tool: " + err.Error())
	}

	listTabs, err := NewListTabsTool(deps.TabMgr)
	if err != nil {
		panic("failed to create list_tabs tool: " + err.Error())
	}

	switchTab, err := NewSwitchTabTool(deps.TabMgr)
	if err != nil {
		panic("failed to create switch_tab tool: " + err.Error())
	}

	openTab, err := NewOpenTabTool(deps.WSManager, deps.Waiter)
	if err != nil {
		panic("failed to create open_tab tool: " + err.Error())
	}

	r.tools = []tool.BaseTool{
		getPageState, executeAction, searchMemory, saveMemory,
		askHuman, listTabs, switchTab, openTab,
	}
}
