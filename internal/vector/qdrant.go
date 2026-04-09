package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Qdrant 向量数据库客户端（使用 REST API）
type Qdrant struct {
	baseURL    string
	collection string
	client     *http.Client
}

// NewQdrant 创建 Qdrant 客户端
func NewQdrant(baseURL, collection string) *Qdrant {
	return &Qdrant{
		baseURL:    baseURL,
		collection: collection,
		client:     &http.Client{},
	}
}

// EnsureCollection 确保集合存在
func (q *Qdrant) EnsureCollection(ctx context.Context, dim int) error {
	// 检查集合是否存在
	url := fmt.Sprintf("%s/collections/%s", q.baseURL, q.collection)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("检查 Qdrant 集合失败: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil // 已存在
	}

	// 创建集合
	body := map[string]any{
		"vectors": map[string]any{
			"size":     dim,
			"distance": "Cosine",
		},
	}
	data, _ := json.Marshal(body)
	req, _ = http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err = q.client.Do(req)
	if err != nil {
		return fmt.Errorf("创建 Qdrant 集合失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("创建 Qdrant 集合失败: %s", string(b))
	}
	return nil
}

// Upsert 插入或更新向量
func (q *Qdrant) Upsert(ctx context.Context, id string, vector []float32, payload map[string]any) error {
	url := fmt.Sprintf("%s/collections/%s/points", q.baseURL, q.collection)
	body := map[string]any{
		"points": []map[string]any{
			{
				"id":      id,
				"vector":  vector,
				"payload": payload,
			},
		},
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("Qdrant upsert 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant upsert 失败: %s", string(b))
	}
	return nil
}

// SearchResult 搜索结果
type SearchResult struct {
	ID      string         `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

// Search 语义搜索
func (q *Qdrant) Search(ctx context.Context, vector []float32, limit int, filter map[string]any) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/collections/%s/points/search", q.baseURL, q.collection)
	body := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}
	if filter != nil {
		body["filter"] = filter
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Qdrant 搜索失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result []struct {
			ID      string         `json:"id"`
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 Qdrant 响应失败: %w", err)
	}

	var results []SearchResult
	for _, r := range result.Result {
		results = append(results, SearchResult{
			ID:      r.ID,
			Score:   r.Score,
			Payload: r.Payload,
		})
	}
	return results, nil
}

// Delete 删除向量
func (q *Qdrant) Delete(ctx context.Context, ids []string) error {
	url := fmt.Sprintf("%s/collections/%s/points/delete", q.baseURL, q.collection)
	body := map[string]any{
		"points": ids,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("Qdrant 删除失败: %w", err)
	}
	resp.Body.Close()
	return nil
}
