package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gentpan/recalla/internal/compress"
	"github.com/gentpan/recalla/internal/memory"
	"github.com/google/uuid"
)

// Server MCP 协议服务器（Streamable HTTP 传输）
type Server struct {
	mem        *memory.Service
	compressor *compress.Compressor
	mu         sync.Mutex
	sessions   map[string]bool // 跟踪活跃会话
}

// NewServer 创建 MCP 服务器
func NewServer(mem *memory.Service, compressor *compress.Compressor) *Server {
	return &Server{
		mem:        mem,
		compressor: compressor,
		sessions:   make(map[string]bool),
	}
}

// JSON-RPC 2.0 消息结构
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServeHTTP 处理 MCP 请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		// 关闭会话
		sessionID := r.Header.Get("Mcp-Session-Id")
		if sessionID != "" {
			s.mu.Lock()
			delete(s.sessions, sessionID)
			s.mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == "GET" {
		// SSE 端点（用于服务器推送）
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		if sessionID := r.Header.Get("Mcp-Session-Id"); sessionID != "" {
			w.Header().Set("Mcp-Session-Id", sessionID)
		}
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "解析错误")
		return
	}

	log.Printf("[MCP] 方法: %s, id: %v", req.Method, req.ID)

	switch req.Method {
	case "initialize":
		s.handleInitialize(w, &req)
	case "tools/list":
		// 返回已有的 session id
		if sessionID := r.Header.Get("Mcp-Session-Id"); sessionID != "" {
			w.Header().Set("Mcp-Session-Id", sessionID)
		}
		s.handleToolsList(w, &req)
	case "tools/call":
		if sessionID := r.Header.Get("Mcp-Session-Id"); sessionID != "" {
			w.Header().Set("Mcp-Session-Id", sessionID)
		}
		s.handleToolsCall(w, r, &req)
	case "notifications/initialized":
		// 客户端确认初始化完成，不需要响应
		w.WriteHeader(http.StatusAccepted)
	default:
		writeRPCError(w, req.ID, -32601, fmt.Sprintf("未知方法: %s", req.Method))
	}
}

// 处理初始化请求
func (s *Server) handleInitialize(w http.ResponseWriter, req *jsonRPCRequest) {
	// 生成会话 ID
	sessionID := uuid.New().String()
	s.mu.Lock()
	s.sessions[sessionID] = true
	s.mu.Unlock()
	w.Header().Set("Mcp-Session-Id", sessionID)
	log.Printf("[MCP] 新会话: %s", sessionID)

	writeRPCResult(w, req.ID, map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "recalla",
			"version": "0.1.0",
		},
	})
}

// 处理工具列表请求
func (s *Server) handleToolsList(w http.ResponseWriter, req *jsonRPCRequest) {
	writeRPCResult(w, req.ID, map[string]any{
		"tools": getToolDefinitions(),
	})
}

// 处理工具调用请求
func (s *Server) handleToolsCall(w http.ResponseWriter, r *http.Request, req *jsonRPCRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "参数格式错误")
		return
	}

	userID := "default"
	result, err := s.callTool(r, userID, params.Name, params.Arguments)
	if err != nil {
		writeRPCResult(w, req.ID, map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("错误: %v", err)},
			},
			"isError": true,
		})
		return
	}

	writeRPCResult(w, req.ID, map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": result},
		},
	})
}

// callTool 执行工具调用
func (s *Server) callTool(r *http.Request, userID, name string, args json.RawMessage) (string, error) {
	ctx := r.Context()

	switch name {
	case "memory_save":
		var req memory.SaveRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		mem, err := s.mem.Save(ctx, userID, req)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("记忆已保存，ID: %s", mem.ID), nil

	case "memory_search":
		var req memory.SearchRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		memories, err := s.mem.Search(ctx, userID, req)
		if err != nil {
			return "", err
		}
		if len(memories) == 0 {
			return "没有找到相关记忆。", nil
		}
		data, _ := json.MarshalIndent(memories, "", "  ")
		return string(data), nil

	case "context_restore":
		var req memory.ContextRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		ctx_resp, err := s.mem.GetContext(ctx, userID, req)
		if err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(ctx_resp, "", "  ")
		return string(data), nil

	case "session_sync":
		var req memory.SessionSyncRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		if err := s.mem.SyncSession(ctx, userID, req); err != nil {
			return "", err
		}
		return "会话已同步到云端。", nil

	case "session_compress":
		var req struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		compressed, err := s.compressor.Compress(ctx, req.Content)
		if err != nil {
			return "", err
		}
		return compressed, nil

	case "project_list":
		projects, err := s.mem.ListProjects(ctx, userID)
		if err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(projects, "", "  ")
		return string(data), nil

	case "config_push":
		var req struct {
			FileKey string `json:"file_key"`
			Content string `json:"content"`
			Device  string `json:"device"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		if err := s.mem.ConfigPush(ctx, userID, req.FileKey, req.Content, req.Device); err != nil {
			return "", err
		}
		return fmt.Sprintf("配置 %s 已推送到服务器。", req.FileKey), nil

	case "config_pull":
		var req struct {
			FileKey string `json:"file_key"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		cfg, err := s.mem.ConfigPull(ctx, userID, req.FileKey)
		if err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		return string(data), nil

	case "team_search":
		var req struct {
			TeamID string `json:"team_id"`
			Query  string `json:"query"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		memories, err := s.mem.SearchTeamMemories(ctx, userID, req.TeamID, req.Query, req.Limit)
		if err != nil { return "", err }
		if len(memories) == 0 { return "团队中没有找到相关记忆。", nil }
		data, _ := json.MarshalIndent(memories, "", "  ")
		return string(data), nil

	case "team_share":
		var req struct {
			TeamID   string `json:"team_id"`
			MemoryID string `json:"memory_id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		if err := s.mem.ShareMemory(ctx, req.TeamID, req.MemoryID, userID); err != nil { return "", err }
		return "记忆已共享到团队。", nil

	case "add_fact":
		var req memory.EntityFact
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		fact, err := s.mem.AddFact(ctx, userID, req)
		if err != nil { return "", err }
		return fmt.Sprintf("事实已记录：%s %s %s (ID: %s)", fact.Subject, fact.Predicate, fact.Object, fact.ID[:8]), nil

	case "query_facts":
		var req struct {
			Subject   string `json:"subject"`
			Predicate string `json:"predicate"`
			Project   string `json:"project"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return "", fmt.Errorf("参数格式错误: %w", err)
		}
		facts, err := s.mem.QueryFacts(ctx, userID, req.Subject, req.Predicate, req.Project, false)
		if err != nil { return "", err }
		if len(facts) == 0 { return "没有找到相关事实。", nil }
		data, _ := json.MarshalIndent(facts, "", "  ")
		return string(data), nil

	default:
		return "", fmt.Errorf("未知工具: %s", name)
	}
}

func writeRPCResult(w http.ResponseWriter, id any, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeRPCError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	})
}
