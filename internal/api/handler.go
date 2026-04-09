package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"fmt"

	"github.com/gentpan/recalla/internal/auth"
	"github.com/gentpan/recalla/internal/compress"
	"github.com/gentpan/recalla/internal/config"
	ghpkg "github.com/gentpan/recalla/internal/github"
	tgpkg "github.com/gentpan/recalla/internal/telegram"
	"github.com/gentpan/recalla/internal/memory"
)

// Handler API 处理器
type Handler struct {
	mem        *memory.Service
	compressor *compress.Compressor
	cfg        *config.Config
	auth       *auth.Service
	tgBot      *tgpkg.Bot
}

// NewHandler 创建处理器
func NewHandler(mem *memory.Service, compressor *compress.Compressor, cfg *config.Config, authSvc *auth.Service) *Handler {
	return &Handler{mem: mem, compressor: compressor, cfg: cfg, auth: authSvc}
}

// SetTelegramBot 设置 Telegram Bot（用于 GitHub → Telegram 通知）
func (h *Handler) SetTelegramBot(bot *tgpkg.Bot) { h.tgBot = bot }

// Register 注册路由
func (h *Handler) Register(mux *http.ServeMux) {
	// 公开接口（不需要认证）
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("GET /api/health", h.health)

	// 需要认证的接口
	mux.HandleFunc("POST /api/memory/save", h.saveMemory)
	mux.HandleFunc("POST /api/memory/search", h.searchMemory)
	mux.HandleFunc("DELETE /api/memory/{id}", h.deleteMemory)
	mux.HandleFunc("POST /api/context/restore", h.restoreContext)
	mux.HandleFunc("POST /api/session/sync", h.syncSession)
	mux.HandleFunc("POST /api/session/compress", h.compressSession)
	mux.HandleFunc("GET /api/projects", h.listProjects)
	mux.HandleFunc("GET /api/stats", h.stats)
	mux.HandleFunc("GET /api/sessions", h.listSessions)
	mux.HandleFunc("GET /api/project/{name}", h.projectDetail)
	mux.HandleFunc("GET /api/project/{name}/memories", h.projectMemories)
	mux.HandleFunc("GET /api/project/{name}/sessions", h.projectSessions)
	mux.HandleFunc("PUT /api/memory/{id}", h.updateMemory)
	mux.HandleFunc("GET /api/settings", h.getSettings)
	mux.HandleFunc("POST /api/settings", h.saveSettings)
	mux.HandleFunc("POST /api/sessions/import", h.importSessions)

	// Config Sync
	mux.HandleFunc("POST /api/config/push", h.configPush)
	mux.HandleFunc("POST /api/config/pull", h.configPull)
	mux.HandleFunc("GET /api/config/list", h.configList)

	// Knowledge Graph
	mux.HandleFunc("POST /api/kg/facts", h.addFact)
	mux.HandleFunc("GET /api/kg/facts", h.queryFacts)
	mux.HandleFunc("DELETE /api/kg/facts/{id}", h.invalidateFact)
	mux.HandleFunc("POST /api/memory/check", h.checkContradiction)

	// Teams
	mux.HandleFunc("POST /api/teams", h.createTeam)
	mux.HandleFunc("GET /api/teams", h.listTeams)
	mux.HandleFunc("GET /api/teams/{id}/members", h.listTeamMembers)
	mux.HandleFunc("POST /api/teams/{id}/members", h.addTeamMember)
	mux.HandleFunc("DELETE /api/teams/{id}/members/{uid}", h.removeTeamMember)
	mux.HandleFunc("POST /api/teams/{id}/share", h.shareMemory)
	mux.HandleFunc("GET /api/teams/{id}/memories", h.listSharedMemories)
	mux.HandleFunc("GET /api/teams/{id}/detail", h.teamDetail)
	mux.HandleFunc("POST /api/teams/{id}/invite", h.inviteMember)
	mux.HandleFunc("POST /api/teams/{id}/projects", h.addTeamProject)
	mux.HandleFunc("GET /api/teams/{id}/projects", h.listTeamProjects)
	mux.HandleFunc("POST /api/teams/{id}/search", h.searchTeamMemories)
	mux.HandleFunc("GET /api/invites", h.listInvites)
	mux.HandleFunc("POST /api/invites/{id}/accept", h.acceptInvite)

	// Briefing
	mux.HandleFunc("POST /api/briefing", h.generateBriefing)

	// GitHub
	mux.HandleFunc("GET /api/github/repos", h.githubRepos)
	mux.HandleFunc("GET /api/github/repos/{owner}/{repo}/commits", h.githubCommits)
	mux.HandleFunc("POST /api/github/webhook", h.githubWebhook)

	// 账户管理
	mux.HandleFunc("GET /api/auth/me", h.me)
	mux.HandleFunc("POST /api/auth/password", h.changePassword)
	mux.HandleFunc("POST /api/auth/username", h.changeUsername)
	mux.HandleFunc("GET /api/auth/keys", h.listKeys)
	mux.HandleFunc("POST /api/auth/keys", h.createKey)
	mux.HandleFunc("DELETE /api/auth/keys/{id}", h.deleteKey)
}

// 保存记忆
func (h *Handler) saveMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	var req memory.SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content 不能为空")
		return
	}
	if req.Project == "" {
		writeError(w, http.StatusBadRequest, "project 不能为空")
		return
	}

	mem, err := h.mem.Save(r.Context(), userID, req)
	if err != nil {
		log.Printf("保存记忆失败: %v", err)
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}
	writeJSON(w, http.StatusOK, mem)
}

// 搜索记忆
func (h *Handler) searchMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	var req memory.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query 不能为空")
		return
	}

	memories, err := h.mem.Search(r.Context(), userID, req)
	if err != nil {
		log.Printf("搜索记忆失败: %v", err)
		writeError(w, http.StatusInternalServerError, "搜索失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"results": memories,
		"count":   len(memories),
	})
}

// 删除记忆
func (h *Handler) deleteMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	memoryID := r.PathValue("id")
	if memoryID == "" {
		writeError(w, http.StatusBadRequest, "id 不能为空")
		return
	}

	if err := h.mem.Delete(r.Context(), userID, memoryID); err != nil {
		log.Printf("删除记忆失败: %v", err)
		writeError(w, http.StatusInternalServerError, "删除失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// 恢复项目上下文
func (h *Handler) restoreContext(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	var req memory.ContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Project == "" {
		writeError(w, http.StatusBadRequest, "project 不能为空")
		return
	}

	ctx, err := h.mem.GetContext(r.Context(), userID, req)
	if err != nil {
		log.Printf("恢复上下文失败: %v", err)
		writeError(w, http.StatusInternalServerError, "恢复失败")
		return
	}
	writeJSON(w, http.StatusOK, ctx)
}

// 同步会话
func (h *Handler) syncSession(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	var req memory.SessionSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	if err := h.mem.SyncSession(r.Context(), userID, req); err != nil {
		log.Printf("同步会话失败: %v", err)
		writeError(w, http.StatusInternalServerError, "同步失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// 压缩会话（压缩后自动保存为记忆）
func (h *Handler) compressSession(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }

	var req struct {
		Content string `json:"content"`
		Project string `json:"project,omitempty"`
		Mode    string `json:"mode,omitempty"` // brief, structured, detailed
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		writeError(w, http.StatusBadRequest, "content 不能为空")
		return
	}
	if req.Mode == "" { req.Mode = "structured" }

	compressed, err := h.compressor.CompressWithMode(r.Context(), req.Content, req.Mode)
	if err != nil {
		log.Printf("压缩失败: %v", err)
		writeError(w, http.StatusInternalServerError, "压缩失败")
		return
	}

	// 自动保存压缩结果为记忆
	var memoryID string
	if req.Project != "" {
		mem, err := h.mem.Save(r.Context(), userID, memory.SaveRequest{
			Project:    req.Project,
			Type:       "session",
			Content:    compressed,
			Tags:       []string{"compressed", "session-summary"},
			Importance: 0.6,
		})
		if err == nil { memoryID = mem.ID }
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"compressed": compressed,
		"memory_id":  memoryID,
	})
}

// 列出项目
func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}

	projects, err := h.mem.ListProjects(r.Context(), userID)
	if err != nil {
		log.Printf("列出项目失败: %v", err)
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

// 项目详情
func (h *Handler) projectDetail(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	name := r.PathValue("name")
	detail, err := h.mem.GetProjectDetail(r.Context(), userID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取项目详情失败")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// 项目记忆列表
func (h *Handler) projectMemories(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	name := r.PathValue("name")
	memType := r.URL.Query().Get("type")
	memories, err := h.mem.ListMemoriesByProject(r.Context(), userID, name, memType, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memories": memories, "count": len(memories)})
}

// 项目会话列表
func (h *Handler) projectSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	name := r.PathValue("name")
	sessions, err := h.mem.ListSessionsByProject(r.Context(), userID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions, "count": len(sessions)})
}

// 更新记忆
func (h *Handler) updateMemory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	memoryID := r.PathValue("id")
	var req struct {
		Content string   `json:"content"`
		Type    string   `json:"type"`
		Tags    []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := h.mem.UpdateMemory(r.Context(), userID, memoryID, req.Content, req.Type, req.Tags); err != nil {
		writeError(w, http.StatusInternalServerError, "更新失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// 统计数据
func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}
	stats, err := h.mem.GetStats(r.Context(), userID)
	if err != nil {
		log.Printf("获取统计失败: %v", err)
		writeError(w, http.StatusInternalServerError, "获取统计失败")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// 列出会话
func (h *Handler) listSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "default"
	}
	sessions, err := h.mem.ListSessions(r.Context(), userID)
	if err != nil {
		log.Printf("列出会话失败: %v", err)
		writeError(w, http.StatusInternalServerError, "查询失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// 获取设置
func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	// 返回提供商列表（隐藏 key）
	maskedProviders := make([]map[string]any, len(h.cfg.Providers))
	for i, p := range h.cfg.Providers {
		maskedProviders[i] = map[string]any{
			"name":          p.Name,
			"url":           p.URL,
			"key":           maskKey(p.Key),
			"embed_model":   p.EmbedModel,
			"llm_model":     p.LLMModel,
			"embedding_dim": p.EmbeddingDim,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers":        maskedProviders,
		"active_embedding": h.cfg.ActiveEmbedding,
		"active_llm":       h.cfg.ActiveLLM,
		"github_token":     maskKey(h.cfg.GitHubToken),
		"github_owner":     h.cfg.GitHubOwner,
		"github_webhook_secret": maskKey(h.cfg.GitHubWebhookSecret),
	})
}

// 保存设置
func (h *Handler) saveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Providers       []config.AIProvider `json:"providers"`
		ActiveEmbedding string              `json:"active_embedding"`
		ActiveLLM       string              `json:"active_llm"`
		GitHubToken     string              `json:"github_token"`
		GitHubOwner     string              `json:"github_owner"`
		GitHubWebhook   string              `json:"github_webhook_secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	// 处理 masked key：如果前端返回 masked 值，保留旧值
	for i := range req.Providers {
		if strings.Contains(req.Providers[i].Key, "***") {
			// 查找旧提供商的 key
			for _, old := range h.cfg.Providers {
				if old.Name == req.Providers[i].Name {
					req.Providers[i].Key = old.Key
					break
				}
			}
		}
	}

	h.cfg.UpdateProviders(req.Providers, req.ActiveEmbedding, req.ActiveLLM)

	if req.GitHubToken != "" && !strings.Contains(req.GitHubToken, "***") {
		h.cfg.GitHubToken = req.GitHubToken
	}
	if req.GitHubOwner != "" {
		h.cfg.GitHubOwner = req.GitHubOwner
	}
	if req.GitHubWebhook != "" && !strings.Contains(req.GitHubWebhook, "***") {
		h.cfg.GitHubWebhookSecret = req.GitHubWebhook
	}

	if err := h.cfg.SaveToFile(); err != nil {
		log.Printf("保存配置失败: %v", err)
		writeError(w, http.StatusInternalServerError, "保存失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// 导入会话（从本地 Claude session JSONL 文件批量导入）
func (h *Handler) importSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }

	var req struct {
		Sessions []struct {
			Project   string `json:"project"`
			SessionID string `json:"session_id"`
			Content   string `json:"content"`
			Device    string `json:"device"`
			Branch    string `json:"branch"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	imported := 0
	for _, s := range req.Sessions {
		err := h.mem.SyncSession(r.Context(), userID, memory.SessionSyncRequest{
			Project:   s.Project,
			SessionID: s.SessionID,
			Content:   s.Content,
			Device:    s.Device,
			Branch:    s.Branch,
		})
		if err != nil {
			log.Printf("导入会话失败: %v", err)
			continue
		}
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "total": len(req.Sessions)})
}

// Config: 推送配置到服务器
func (h *Handler) configPush(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	var req struct {
		FileKey string `json:"file_key"`
		Content string `json:"content"`
		Device  string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileKey == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "file_key and content required")
		return
	}
	if err := h.mem.ConfigPush(r.Context(), userID, req.FileKey, req.Content, req.Device); err != nil {
		writeError(w, http.StatusInternalServerError, "push failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Config: 从服务器拉取配置
func (h *Handler) configPull(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	var req struct {
		FileKey string `json:"file_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileKey == "" {
		writeError(w, http.StatusBadRequest, "file_key required")
		return
	}
	cfg, err := h.mem.ConfigPull(r.Context(), userID, req.FileKey)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// Config: 列出所有配置
func (h *Handler) configList(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	configs, err := h.mem.ConfigList(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configs": configs})
}

// ========== Teams ==========

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	var req struct { Name string `json:"name"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" { writeError(w, http.StatusBadRequest, "name required"); return }
	team, err := h.mem.CreateTeam(r.Context(), req.Name, user.ID)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, team)
}

func (h *Handler) listTeams(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	teams, err := h.mem.ListTeams(r.Context(), user.ID)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

func (h *Handler) listTeamMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.mem.ListTeamMembers(r.Context(), r.PathValue("id"))
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) addTeamMember(w http.ResponseWriter, r *http.Request) {
	var req struct { UserID string `json:"user_id"`; Role string `json:"role"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Role == "" { req.Role = "member" }
	if err := h.mem.AddTeamMember(r.Context(), r.PathValue("id"), req.UserID, req.Role); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error()); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) removeTeamMember(w http.ResponseWriter, r *http.Request) {
	if err := h.mem.RemoveTeamMember(r.Context(), r.PathValue("id"), r.PathValue("uid")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error()); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) shareMemory(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	var req struct { MemoryID string `json:"memory_id"` }
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.mem.ShareMemory(r.Context(), r.PathValue("id"), req.MemoryID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error()); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listSharedMemories(w http.ResponseWriter, r *http.Request) {
	memories, err := h.mem.ListSharedMemories(r.Context(), r.PathValue("id"), 50)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"memories": memories})
}

func (h *Handler) teamDetail(w http.ResponseWriter, r *http.Request) {
	detail, err := h.mem.GetTeamDetail(r.Context(), r.PathValue("id"))
	if err != nil { writeError(w, http.StatusNotFound, err.Error()); return }
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) inviteMember(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	var req struct { Username string `json:"username"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Username == "" { writeError(w, http.StatusBadRequest, "username required"); return }
	invite, err := h.mem.InviteMember(r.Context(), r.PathValue("id"), user.ID, req.Username)
	if err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }
	writeJSON(w, http.StatusOK, invite)
}

func (h *Handler) addTeamProject(w http.ResponseWriter, r *http.Request) {
	var req struct { Project string `json:"project"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Project == "" { writeError(w, http.StatusBadRequest, "project required"); return }
	if err := h.mem.AddTeamProject(r.Context(), r.PathValue("id"), req.Project); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error()); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listTeamProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.mem.ListTeamProjects(r.Context(), r.PathValue("id"))
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (h *Handler) searchTeamMemories(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	var req struct { Query string `json:"query"`; Limit int `json:"limit"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Query == "" { writeError(w, http.StatusBadRequest, "query required"); return }
	memories, err := h.mem.SearchTeamMemories(r.Context(), userID, r.PathValue("id"), req.Query, req.Limit)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"results": memories, "count": len(memories)})
}

func (h *Handler) listInvites(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	invites, err := h.mem.ListPendingInvites(r.Context(), user.Username)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"invites": invites})
}

func (h *Handler) acceptInvite(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	if err := h.mem.AcceptInvite(r.Context(), r.PathValue("id"), user.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error()); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ========== Briefing ==========

func (h *Handler) generateBriefing(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	var req struct { Project string `json:"project"` }
	json.NewDecoder(r.Body).Decode(&req)
	briefing, err := h.mem.GenerateBriefing(r.Context(), userID, req.Project)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, briefing)
}

// ========== Knowledge Graph ==========

func (h *Handler) addFact(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	var fact memory.EntityFact
	if err := json.NewDecoder(r.Body).Decode(&fact); err != nil {
		writeError(w, http.StatusBadRequest, "bad request"); return
	}
	if fact.Subject == "" || fact.Predicate == "" || fact.Object == "" {
		writeError(w, http.StatusBadRequest, "subject, predicate, object required"); return
	}
	result, err := h.mem.AddFact(r.Context(), userID, fact)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) queryFacts(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	facts, err := h.mem.QueryFacts(r.Context(), userID,
		r.URL.Query().Get("subject"),
		r.URL.Query().Get("predicate"),
		r.URL.Query().Get("project"),
		r.URL.Query().Get("expired") == "true")
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"facts": facts})
}

func (h *Handler) invalidateFact(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	if err := h.mem.InvalidateFact(r.Context(), userID, r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error()); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) checkContradiction(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" { userID = "default" }
	var req struct { Content string `json:"content"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Content == "" { writeError(w, http.StatusBadRequest, "content required"); return }
	similar, err := h.mem.CheckContradiction(r.Context(), userID, req.Content)
	if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }
	writeJSON(w, http.StatusOK, map[string]any{"similar_memories": similar, "count": len(similar)})
}

// GitHub: 获取仓库列表
func (h *Handler) githubRepos(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GitHubToken == "" {
		writeError(w, http.StatusBadRequest, "GitHub Token 未配置")
		return
	}
	client := ghpkg.NewClient(h.cfg.GitHubToken)
	repos, err := client.ListRepos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repos": repos})
}

// GitHub: 获取仓库提交历史
func (h *Handler) githubCommits(w http.ResponseWriter, r *http.Request) {
	if h.cfg.GitHubToken == "" {
		writeError(w, http.StatusBadRequest, "GitHub Token 未配置")
		return
	}
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	client := ghpkg.NewClient(h.cfg.GitHubToken)
	commits, err := client.ListCommits(r.Context(), owner, repo, 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commits": commits})
}

// GitHub: Webhook 接收（push/PR 自动保存记忆）
func (h *Handler) githubWebhook(w http.ResponseWriter, r *http.Request) {
	event, err := ghpkg.ParseWebhook(r, h.cfg.GitHubWebhookSecret)
	if err != nil {
		log.Printf("Webhook 解析失败: %v", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[Webhook] %s on %s", event.Type, event.Repo)

	// 从仓库名提取项目名（取最后一段）
	project := event.Repo
	if parts := strings.Split(event.Repo, "/"); len(parts) > 1 {
		project = parts[len(parts)-1]
	}

	switch event.Type {
	case "push":
		// 记录每次 push 为记忆
		var content string
		for _, c := range event.Commits {
			files := append(append(c.Added, c.Modified...), c.Removed...)
			content += fmt.Sprintf("[%s] %s (%s)\n  files: %s\n",
				c.SHA, c.Message, c.Author, strings.Join(files, ", "))
		}
		_, err := h.mem.Save(r.Context(), "default", memory.SaveRequest{
			Project: project,
			Type:    "code",
			Content: fmt.Sprintf("GitHub Push to %s (branch: %s)\n\n%s", event.Repo, event.Branch, content),
			Tags:    []string{"github", "push", event.Branch},
			Metadata: memory.Map{
				"repo":   event.Repo,
				"branch": event.Branch,
				"source": "github-webhook",
			},
		})
		if err != nil {
			log.Printf("保存 push 记忆失败: %v", err)
		}

	case "pull_request":
		if event.PR != nil && (event.PR.Action == "closed" && event.PR.Merged || event.PR.Action == "opened") {
			action := "opened"
			if event.PR.Merged {
				action = "merged"
			}
			content := fmt.Sprintf("PR #%d %s: %s\nAuthor: %s\n\n%s",
				event.PR.Number, action, event.PR.Title, event.PR.Author, event.PR.Body)
			_, err := h.mem.Save(r.Context(), "default", memory.SaveRequest{
				Project: project,
				Type:    "code",
				Content: content,
				Tags:    []string{"github", "pull-request", action},
				Metadata: memory.Map{
					"repo":      event.Repo,
					"pr_number": event.PR.Number,
					"source":    "github-webhook",
				},
			})
			if err != nil {
				log.Printf("保存 PR 记忆失败: %v", err)
			}
		}
	}

	// Telegram 通知
	if h.tgBot != nil {
		var msg string
		switch event.Type {
		case "push":
			msg = fmt.Sprintf("[GitHub] Push to *%s* (%s)\n", event.Repo, event.Branch)
			for _, c := range event.Commits {
				msg += fmt.Sprintf("  `%s` %s\n", c.SHA, c.Message)
			}
		case "pull_request":
			if event.PR != nil {
				msg = fmt.Sprintf("[GitHub] PR #%d %s: *%s*\nBy: %s", event.PR.Number, event.PR.Action, event.PR.Title, event.PR.Author)
			}
		}
		if msg != "" {
			go h.tgBot.NotifyOwner(msg)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// maskKey 隐藏 key 的中间部分
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + "***" + key[len(key)-4:]
}

// 健康检查
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "recalla",
	})
}

// 登录
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	user, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	token := h.auth.GenerateToken(user.ID, user.Username, user.Role)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

// 获取当前用户信息
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	writeJSON(w, http.StatusOK, user)
}

// 修改密码
func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误"); return
	}
	if len(req.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "密码至少 6 位"); return
	}
	if err := h.auth.ChangePassword(r.Context(), user.ID, req.OldPassword, req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error()); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// 修改用户名
func (h *Handler) changeUsername(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	var req struct { Username string `json:"username"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		writeError(w, http.StatusBadRequest, "用户名不能为空"); return
	}
	if err := h.auth.ChangeUsername(r.Context(), user.ID, req.Username); err != nil {
		writeError(w, http.StatusBadRequest, "修改失败"); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// 列出 API Key
func (h *Handler) listKeys(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	keys, err := h.auth.ListAPIKeys(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询失败"); return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

// 生成 API Key
func (h *Handler) createKey(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	var req struct { Name string `json:"name"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" { req.Name = "default" }
	key, err := h.auth.GenerateAPIKey(r.Context(), user.ID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成失败"); return
	}
	writeJSON(w, http.StatusOK, key)
}

// 删除 API Key
func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	user := getUserFromCtx(r)
	if user == nil { writeError(w, http.StatusUnauthorized, "未登录"); return }
	keyID := r.PathValue("id")
	if err := h.auth.DeleteAPIKey(r.Context(), user.ID, keyID); err != nil {
		writeError(w, http.StatusInternalServerError, "删除失败"); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type ctxKey string

func getUserFromCtx(r *http.Request) *auth.User {
	u, _ := r.Context().Value(ctxKey("user")).(*auth.User)
	return u
}

// AuthMiddleware 认证中间件（支持 token 和 API Key）
func AuthMiddleware(authSvc *auth.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 不需要认证的路径
		if r.URL.Path == "/api/health" || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/github/webhook" || r.URL.Path == "/api/telegram/webhook" {
			next.ServeHTTP(w, r)
			return
		}
		// MCP 端点用 API Key 认证
		if strings.HasPrefix(r.URL.Path, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}
		// 静态文件不需要认证
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "需要认证")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// 尝试 API Key 认证
		if strings.HasPrefix(token, "rk-") {
			userID, err := authSvc.ValidateAPIKey(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "API Key 无效")
				return
			}
			r.Header.Set("X-User-ID", userID)
			next.ServeHTTP(w, r)
			return
		}

		// 尝试 token 认证
		user, err := authSvc.ValidateToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "认证失效，请重新登录")
			return
		}
		r.Header.Set("X-User-ID", user.ID)
		ctx := r.Context()
		ctx = contextWithUser(ctx, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func contextWithUser(ctx context.Context, user *auth.User) context.Context {
	return context.WithValue(ctx, ctxKey("user"), user)
}

// CORS 中间件
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
