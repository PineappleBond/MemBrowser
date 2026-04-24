package tools

import "encoding/json"

type GetPageStateInput struct {
	IncludeScreenshot bool `json:"include_screenshot,omitempty"`
}

type GetPageStateOutput struct {
	URL        string          `json:"url"`
	Title      string          `json:"title"`
	DOMSummary json.RawMessage `json:"dom_summary"`
	Screenshot string          `json:"screenshot,omitempty"`
}

type ExecuteActionInput struct {
	Action   string `json:"action" jsonschema:"enum=click,enum=input,enum=scroll,enum=navigate"`
	Selector string `json:"selector,omitempty"`
	Value    string `json:"value,omitempty"`
	URL      string `json:"url,omitempty"`
}

type ExecuteActionOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type SearchMemoryInput struct {
	PageURL    string `json:"page_url"`
	ActionType string `json:"action_type,omitempty"`
}

type SearchMemoryOutput struct {
	Found  bool   `json:"found"`
	Memory string `json:"memory,omitempty"`
}

type SaveMemoryInput struct {
	PageURL      string `json:"page_url"`
	ActionType   string `json:"action_type"`
	ActionTarget string `json:"action_target"`
	Result       string `json:"result"`
}

type SaveMemoryOutput struct {
	Success bool `json:"success"`
}

type ListTabsInput struct{}

type ListTabsOutput struct {
	Tabs []TabInfo `json:"tabs"`
}

type TabInfo struct {
	ID     int    `json:"id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Active bool   `json:"active"`
}

type SwitchTabInput struct {
	TabID int `json:"tab_id"`
}

type SwitchTabOutput struct {
	Success bool `json:"success"`
}

type OpenTabInput struct {
	URL string `json:"url"`
}

type OpenTabOutput struct {
	TabID int `json:"tab_id"`
}
