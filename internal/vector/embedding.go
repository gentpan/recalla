package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// EmbedConfig 嵌入配置接口
type EmbedConfig interface {
	GetEmbeddingConfig() (url, key, model string, dim int)
}

// Embedder 文本嵌入客户端（动态从 Config 获取提供商配置）
type Embedder struct {
	cfg    EmbedConfig
	client *http.Client
}

// NewEmbedder 创建嵌入客户端
func NewEmbedder(cfg EmbedConfig) *Embedder {
	return &Embedder{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// Embed 将文本转换为向量（每次请求动态获取当前 active provider）
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	url, key, model, _ := e.cfg.GetEmbeddingConfig()
	if url == "" || key == "" {
		return nil, fmt.Errorf("embedding 未配置，请在 Settings 中添加 AI 提供商")
	}

	body := map[string]any{
		"input": text,
		"model": model,
	}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API 错误 (%d): %s", resp.StatusCode, string(b))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 embedding 响应失败: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embedding 返回空结果")
	}
	return result.Data[0].Embedding, nil
}
