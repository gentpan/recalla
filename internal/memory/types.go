package memory

import (
	"time"
)

// Memory 记忆条目
type Memory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Project   string    `json:"project"`
	Type      string    `json:"type"`      // conversation, code, decision, note, session
	Content   string    `json:"content"`
	Summary   string    `json:"summary,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Metadata  Map       `json:"metadata,omitempty"` // 额外元数据（git分支、设备名等）
	Score     float64   `json:"score,omitempty"`     // 搜索时的相关性分数
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Map 通用 map 类型
type Map map[string]any

// SaveRequest 保存记忆请求
type SaveRequest struct {
	Project  string   `json:"project"`
	Type     string   `json:"type"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags,omitempty"`
	Metadata Map      `json:"metadata,omitempty"`
}

// SearchRequest 搜索记忆请求
type SearchRequest struct {
	Query   string   `json:"query"`
	Project string   `json:"project,omitempty"`
	Type    string   `json:"type,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Limit   int      `json:"limit,omitempty"`
}

// ContextRequest 获取项目上下文请求
type ContextRequest struct {
	Project string `json:"project"`
	Device  string `json:"device,omitempty"`
}

// ContextResponse 项目上下文响应
type ContextResponse struct {
	Project       string   `json:"project"`
	LastDevice    string   `json:"last_device,omitempty"`
	LastBranch    string   `json:"last_branch,omitempty"`
	LastActivity  string   `json:"last_activity,omitempty"`
	RecentMemories []Memory `json:"recent_memories"`
	Summary       string   `json:"summary,omitempty"`
}

// SessionSyncRequest 会话同步请求
type SessionSyncRequest struct {
	Project   string `json:"project"`
	SessionID string `json:"session_id"`
	Content   string `json:"content"`   // 完整会话内容
	Device    string `json:"device"`
	Branch    string `json:"branch,omitempty"`
}

// CompressRequest 压缩请求
type CompressRequest struct {
	Project string `json:"project"`
	Before  string `json:"before,omitempty"` // 压缩此时间之前的记忆
}
