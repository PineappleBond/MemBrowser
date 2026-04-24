package eino

import (
	"context"

	"github.com/PineappleBond/MemBrowser/backend/internal/config"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

type ModelProvider struct {
	cfg config.Config
}

func NewModelProvider(cfg *config.Config) *ModelProvider {
	return &ModelProvider{cfg: *cfg}
}

func (m *ModelProvider) GetModel(tier string) (model.BaseChatModel, error) {
	var mc config.ModelConfig
	switch tier {
	case "haiku":
		mc = m.cfg.Haiku
	case "sonnet":
		mc = m.cfg.Sonnet
	case "opus":
		mc = m.cfg.Opus
	default:
		mc = m.cfg.Sonnet
	}
	return openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		BaseURL: mc.BaseURL,
		APIKey:  mc.APIKey,
		Model:   mc.Model,
	})
}
