package compress

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// LLMConfig LLM 配置接口
type LLMConfig interface {
	GetLLMConfig() (url, key, model string)
}

// Compressor AI 会话压缩器（动态从 Config 获取提供商配置）
type Compressor struct {
	cfg    LLMConfig
	client *http.Client
}

// NewCompressor 创建压缩器
func NewCompressor(cfg LLMConfig) *Compressor {
	return &Compressor{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// Compress 压缩会话内容，提取关键信息
func (c *Compressor) Compress(ctx context.Context, content string) (string, error) {
	url, key, model := c.cfg.GetLLMConfig()
	if url == "" || key == "" {
		return "", fmt.Errorf("LLM 未配置，请在 Settings 中添加 AI 提供商")
	}

	prompt := `你是一个 AI 会话压缩助手。请将以下 AI 编程会话内容压缩为结构化摘要，保留关键信息：

1. 做了什么（核心决策和代码变更）
2. 遇到的问题和解决方案
3. 未完成的任务
4. 重要的技术决策和原因
5. 当前代码状态（分支、文件等）

输出格式：
## 摘要
[一句话概述]

## 关键决策
- [决策1]
- [决策2]

## 代码变更
- [变更1]
- [变更2]

## 待办
- [待办1]

## 技术笔记
- [笔记1]

---
以下是需要压缩的会话内容：

` + content

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 2000,
	}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("压缩请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("压缩 API 错误 (%d): %s", resp.StatusCode, string(b))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析压缩响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("压缩返回空结果")
	}
	return result.Choices[0].Message.Content, nil
}
