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

// Compressor AI 会话压缩器
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

// 3 种压缩模式的提示词
var modePrompts = map[string]struct {
	prompt    string
	maxTokens int
}{
	"brief": {
		prompt: `你是一个 AI 会话压缩助手。请用 3-5 句话极简概括以下内容的核心结论。
只保留最重要的决策和结果，不需要过程和细节。

格式：直接输出纯文本，不要标题，不要列表。

以下是内容：

`,
		maxTokens: 500,
	},
	"structured": {
		prompt: `你是一个 AI 会话压缩助手。请将以下内容压缩为结构化摘要，保留关键信息：

输出格式：
## 摘要
[一句话概述]

## 关键决策
- [决策1]

## 代码变更
- [变更1]

## 待办
- [待办1]

## 技术笔记
- [笔记1]

（没有的部分可以省略）

以下是内容：

`,
		maxTokens: 2000,
	},
	"detailed": {
		prompt: `你是一个 AI 会话压缩助手。请详细整理以下内容，保留代码片段、具体命令、完整推理过程和技术细节。

输出格式：
## 概述
[2-3 句话概述]

## 详细过程
[按时间顺序记录每个步骤]

## 关键代码/命令
` + "```" + `
[保留重要的代码片段和命令]
` + "```" + `

## 决策与原因
- [决策]: [原因]

## 遇到的问题
- [问题]: [解决方案]

## 待办事项
- [ ] [待办]

## 参考信息
- [文件路径、URL、配置值等]

以下是内容：

`,
		maxTokens: 4000,
	},
}

// CompressWithMode 按指定模式压缩
func (c *Compressor) CompressWithMode(ctx context.Context, content, mode string) (string, error) {
	url, key, model := c.cfg.GetLLMConfig()
	if url == "" || key == "" {
		return "", fmt.Errorf("LLM 未配置，请在 Settings 中添加 AI 提供商")
	}

	mp, ok := modePrompts[mode]
	if !ok {
		mp = modePrompts["structured"]
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": mp.prompt + content},
		},
		"max_tokens": mp.maxTokens,
	}
	return c.callLLM(ctx, url, key, body)
}

// Compress 默认压缩（structured 模式）
func (c *Compressor) Compress(ctx context.Context, content string) (string, error) {
	return c.CompressWithMode(ctx, content, "structured")
}

// callLLM 调用 LLM API
func (c *Compressor) callLLM(ctx context.Context, url, key string, body map[string]any) (string, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 错误 (%d): %s", resp.StatusCode, string(b))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("返回空结果")
	}
	return result.Choices[0].Message.Content, nil
}
