package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Port     int         `mapstructure:"port"`
	DBPath   string      `mapstructure:"db_path"`
	DataDir  string      `mapstructure:"data_dir"`
	AuthKey  string      `mapstructure:"auth_key"`
	LogLevel string      `mapstructure:"log_level"`
	Haiku    ModelConfig `mapstructure:"haiku"`
	Sonnet   ModelConfig `mapstructure:"sonnet"`
	Opus     ModelConfig `mapstructure:"opus"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
}

// Load 加载配置
func Load() *Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("port", 8080)
	viper.SetDefault("db_path", "./data/membrowser.db")
	viper.SetDefault("data_dir", "./data")
	viper.SetDefault("log_level", "debug")

	// 显式绑定环境变量
	viper.BindEnv("auth_key", "MEMBROWSER_AUTH_KEY")
	viper.BindEnv("port", "MEMBROWSER_PORT")
	viper.BindEnv("db_path", "MEMBROWSER_DB_PATH")
	viper.BindEnv("data_dir", "MEMBROWSER_DATA_DIR")
	viper.BindEnv("log_level", "MEMBROWSER_LOG_LEVEL")

	viper.BindEnv("haiku.base_url", "MEMBROWSER_HAIKU_BASE_URL")
	viper.BindEnv("haiku.api_key", "MEMBROWSER_HAIKU_API_KEY")
	viper.BindEnv("haiku.model", "MEMBROWSER_HAIKU_MODEL")

	viper.BindEnv("sonnet.base_url", "MEMBROWSER_SONNET_BASE_URL")
	viper.BindEnv("sonnet.api_key", "MEMBROWSER_SONNET_API_KEY")
	viper.BindEnv("sonnet.model", "MEMBROWSER_SONNET_MODEL")

	viper.BindEnv("opus.base_url", "MEMBROWSER_OPUS_BASE_URL")
	viper.BindEnv("opus.api_key", "MEMBROWSER_OPUS_API_KEY")
	viper.BindEnv("opus.model", "MEMBROWSER_OPUS_MODEL")

	// 加载 .env 文件（静默失败，文件不存在时不报错）
	if err := godotenv.Load("./configs/.env"); err != nil {
		log.Printf("[config] .env file not loaded: %v", err)
	}

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("[config] config file not read: %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Printf("[config] unmarshal failed: %v, using defaults", err)
		// 使用默认值
		cfg = Config{
			Port:     viper.GetInt("port"),
			DBPath:   viper.GetString("db_path"),
			DataDir:  viper.GetString("data_dir"),
			AuthKey:  viper.GetString("auth_key"),
			LogLevel: viper.GetString("log_level"),
			Haiku: ModelConfig{
				BaseURL: viper.GetString("haiku.base_url"),
				APIKey:  viper.GetString("haiku.api_key"),
				Model:   viper.GetString("haiku.model"),
			},
			Sonnet: ModelConfig{
				BaseURL: viper.GetString("sonnet.base_url"),
				APIKey:  viper.GetString("sonnet.api_key"),
				Model:   viper.GetString("sonnet.model"),
			},
			Opus: ModelConfig{
				BaseURL: viper.GetString("opus.base_url"),
				APIKey:  viper.GetString("opus.api_key"),
				Model:   viper.GetString("opus.model"),
			},
		}
	}

	// 确保 data 目录存在
	if cfg.DataDir != "" {
		if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
			log.Printf("[config] failed to create data dir: %v", err)
		}
	}

	return &cfg
}
