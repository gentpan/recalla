package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// AIProvider AI 提供商配置
type AIProvider struct {
	Name         string `json:"name"`          // 提供商标识：openai, qwen, deepseek, ollama, custom
	URL          string `json:"url"`           // API 基础 URL
	Key          string `json:"key"`           // API Key
	EmbedModel   string `json:"embed_model"`   // Embedding 模型名
	LLMModel     string `json:"llm_model"`     // LLM 模型名
	EmbeddingDim int    `json:"embedding_dim"` // 向量维度
}

// Config 全局配置
type Config struct {
	mu sync.RWMutex

	// 服务器地址
	Addr string
	// API 密钥（用于认证，向后兼容）
	APIKey string

	// PostgreSQL 连接字符串
	DatabaseURL string

	// Qdrant 配置
	QdrantURL        string
	QdrantCollection string

	// 多提供商配置
	Providers       []AIProvider `json:"providers"`
	ActiveEmbedding string       `json:"active_embedding"` // 当前用于 embedding 的提供商 name
	ActiveLLM       string       `json:"active_llm"`       // 当前用于 LLM 压缩的提供商 name

	// 向后兼容旧字段（从环境变量迁移）
	EmbeddingURL   string
	EmbeddingKey   string
	EmbeddingModel string
	EmbeddingDim   int
	LLMURL         string
	LLMKey         string
	LLMModel       string

	// GitHub 集成
	GitHubToken         string
	GitHubOwner         string
	GitHubWebhookSecret string

	// Telegram Bot
	TelegramToken string

	// 配置文件路径
	configFile string
}

// Load 从环境变量加载配置
func Load() *Config {
	c := &Config{
		Addr:                envOr("RECALLA_ADDR", ":14200"),
		APIKey:              envOr("RECALLA_API_KEY", ""),
		DatabaseURL:         envOr("RECALLA_DB_URL", "postgres://recalla:recalla2025@localhost:5432/recalla?sslmode=disable"),
		QdrantURL:           envOr("RECALLA_QDRANT_URL", "http://localhost:6333"),
		QdrantCollection:    envOr("RECALLA_QDRANT_COLLECTION", "memories"),
		EmbeddingURL:        envOr("RECALLA_EMBEDDING_URL", ""),
		EmbeddingKey:        envOr("RECALLA_EMBEDDING_KEY", ""),
		EmbeddingModel:      envOr("RECALLA_EMBEDDING_MODEL", "text-embedding-3-small"),
		EmbeddingDim:        envInt("RECALLA_EMBEDDING_DIM", 1024),
		LLMURL:              envOr("RECALLA_LLM_URL", ""),
		LLMKey:              envOr("RECALLA_LLM_KEY", ""),
		LLMModel:            envOr("RECALLA_LLM_MODEL", "gpt-4o-mini"),
		GitHubToken:         envOr("RECALLA_GITHUB_TOKEN", ""),
		GitHubOwner:         envOr("RECALLA_GITHUB_OWNER", ""),
		GitHubWebhookSecret: envOr("RECALLA_GITHUB_WEBHOOK_SECRET", ""),
		TelegramToken:       envOr("RECALLA_TELEGRAM_TOKEN", ""),
		configFile:          "/opt/recalla/config.json",
	}

	// 尝试从 config.json 加载多提供商配置
	c.loadFromFile()

	// 如果没有 providers，从旧环境变量迁移
	if len(c.Providers) == 0 && c.EmbeddingKey != "" {
		c.migrateFromEnv()
	}

	return c
}

// migrateFromEnv 从旧环境变量迁移到多提供商格式
func (c *Config) migrateFromEnv() {
	// 根据 URL 猜测提供商名
	name := "custom"
	if strings.Contains(c.EmbeddingURL, "dashscope") {
		name = "qwen"
	} else if strings.Contains(c.EmbeddingURL, "openai") {
		name = "openai"
	}

	c.Providers = []AIProvider{
		{
			Name:         name,
			URL:          strings.TrimSuffix(strings.TrimSuffix(c.EmbeddingURL, "/embeddings"), "/v1"),
			Key:          c.EmbeddingKey,
			EmbedModel:   c.EmbeddingModel,
			LLMModel:     c.LLMModel,
			EmbeddingDim: c.EmbeddingDim,
		},
	}
	c.ActiveEmbedding = name
	c.ActiveLLM = name
}

// GetEmbeddingConfig 获取当前 active embedding 提供商的配置
func (c *Config) GetEmbeddingConfig() (url, key, model string, dim int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := range c.Providers {
		if c.Providers[i].Name == c.ActiveEmbedding {
			return c.Providers[i].EmbeddingURL(), c.Providers[i].Key, c.Providers[i].EmbedModel, c.Providers[i].EmbeddingDim
		}
	}
	// 回退到旧配置
	return c.EmbeddingURL, c.EmbeddingKey, c.EmbeddingModel, c.EmbeddingDim
}

// GetLLMConfig 获取当前 active LLM 提供商的配置
func (c *Config) GetLLMConfig() (url, key, model string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := range c.Providers {
		if c.Providers[i].Name == c.ActiveLLM {
			return c.Providers[i].LLMURL(), c.Providers[i].Key, c.Providers[i].LLMModel
		}
	}
	return c.LLMURL, c.LLMKey, c.LLMModel
}

// EmbeddingURL 返回嵌入 API 的完整 URL
func (p *AIProvider) EmbeddingURL() string {
	base := strings.TrimRight(p.URL, "/")
	// 如果已经包含完整路径就直接返回
	if strings.HasSuffix(base, "/embeddings") {
		return base
	}
	// 特殊处理各提供商
	switch p.Name {
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1/embeddings"
	case "openai":
		return "https://api.openai.com/v1/embeddings"
	case "ollama":
		return base + "/api/embeddings"
	default:
		if !strings.Contains(base, "/v1") {
			return base + "/v1/embeddings"
		}
		return base + "/embeddings"
	}
}

// LLMURL 返回 LLM API 的完整 URL
func (p *AIProvider) LLMURL() string {
	base := strings.TrimRight(p.URL, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	switch p.Name {
	case "qwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions"
	case "openai":
		return "https://api.openai.com/v1/chat/completions"
	case "deepseek":
		return "https://api.deepseek.com/v1/chat/completions"
	case "ollama":
		return base + "/api/chat"
	default:
		if !strings.Contains(base, "/v1") {
			return base + "/v1/chat/completions"
		}
		return base + "/chat/completions"
	}
}

// UpdateProviders 更新提供商配置
func (c *Config) UpdateProviders(providers []AIProvider, activeEmbed, activeLLM string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Providers = providers
	c.ActiveEmbedding = activeEmbed
	c.ActiveLLM = activeLLM
}

// configJSON 用于 JSON 序列化
type configJSON struct {
	Providers       []AIProvider `json:"providers"`
	ActiveEmbedding string       `json:"active_embedding"`
	ActiveLLM       string       `json:"active_llm"`
	GitHubToken     string       `json:"github_token,omitempty"`
	GitHubOwner     string       `json:"github_owner,omitempty"`
	GitHubWebhook   string       `json:"github_webhook_secret,omitempty"`
}

// loadFromFile 从 config.json 加载
func (c *Config) loadFromFile() {
	data, err := os.ReadFile(c.configFile)
	if err != nil {
		return
	}
	var cfg configJSON
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	c.Providers = cfg.Providers
	c.ActiveEmbedding = cfg.ActiveEmbedding
	c.ActiveLLM = cfg.ActiveLLM
	if cfg.GitHubToken != "" {
		c.GitHubToken = cfg.GitHubToken
	}
	if cfg.GitHubOwner != "" {
		c.GitHubOwner = cfg.GitHubOwner
	}
	if cfg.GitHubWebhook != "" {
		c.GitHubWebhookSecret = cfg.GitHubWebhook
	}
}

// SaveToFile 将配置持久化到 config.json
func (c *Config) SaveToFile() error {
	c.mu.RLock()
	cfg := configJSON{
		Providers:       c.Providers,
		ActiveEmbedding: c.ActiveEmbedding,
		ActiveLLM:       c.ActiveLLM,
		GitHubToken:     c.GitHubToken,
		GitHubOwner:     c.GitHubOwner,
		GitHubWebhook:   c.GitHubWebhookSecret,
	}
	c.mu.RUnlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return os.WriteFile(c.configFile, data, 0600)
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
