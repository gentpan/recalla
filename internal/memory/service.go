package memory

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gentpan/recalla/internal/db"
	"github.com/gentpan/recalla/internal/vector"
	"github.com/google/uuid"
)

// Service 记忆服务
type Service struct {
	db       *db.DB
	qdrant   *vector.Qdrant
	embedder *vector.Embedder
}

// NewService 创建记忆服务
func NewService(db *db.DB, qdrant *vector.Qdrant, embedder *vector.Embedder) *Service {
	return &Service{
		db:       db,
		qdrant:   qdrant,
		embedder: embedder,
	}
}

// Save 保存记忆
func (s *Service) Save(ctx context.Context, userID string, req SaveRequest) (*Memory, error) {
	id := uuid.New().String()
	now := time.Now()

	// 重要性评分默认值
	importance := req.Importance
	if importance <= 0 || importance > 1 {
		importance = 0.5
	}
	// 决策类自动提高重要性
	if req.Type == "decision" && importance < 0.7 {
		importance = 0.7
	}

	// 存入 Postgres
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO memories (id, user_id, project, type, content, tags, metadata, importance, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, id, userID, req.Project, req.Type, req.Content, req.Tags, req.Metadata, importance, now, now)
	if err != nil {
		return nil, fmt.Errorf("保存记忆失败: %w", err)
	}

	// 生成 embedding 并存入 Qdrant
	go func() {
		vec, err := s.embedder.Embed(context.Background(), req.Content)
		if err != nil {
			log.Printf("生成 embedding 失败: %v", err)
			return
		}
		payload := map[string]any{
			"memory_id": id,
			"user_id":   userID,
			"project":   req.Project,
			"type":      req.Type,
			"tags":      req.Tags,
		}
		if err := s.qdrant.Upsert(context.Background(), id, vec, payload); err != nil {
			log.Printf("存入 Qdrant 失败: %v", err)
		}
	}()

	return &Memory{
		ID:         id,
		UserID:     userID,
		Project:    req.Project,
		Type:       req.Type,
		Content:    req.Content,
		Tags:       req.Tags,
		Metadata:   req.Metadata,
		Importance: importance,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// Search 语义搜索记忆
func (s *Service) Search(ctx context.Context, userID string, req SearchRequest) ([]Memory, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	// 生成查询向量
	vec, err := s.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("生成查询 embedding 失败: %w", err)
	}

	// 构建 Qdrant 过滤条件
	must := []map[string]any{
		{"key": "user_id", "match": map[string]any{"value": userID}},
	}
	if req.Project != "" {
		must = append(must, map[string]any{
			"key": "project", "match": map[string]any{"value": req.Project},
		})
	}
	if req.Type != "" {
		must = append(must, map[string]any{
			"key": "type", "match": map[string]any{"value": req.Type},
		})
	}
	filter := map[string]any{"must": must}

	// Qdrant 语义搜索
	results, err := s.qdrant.Search(ctx, vec, limit, filter)
	if err != nil {
		return nil, fmt.Errorf("向量搜索失败: %w", err)
	}

	if len(results) == 0 {
		return []Memory{}, nil
	}

	// 从 Postgres 获取完整记忆
	var ids []string
	scoreMap := make(map[string]float64)
	for _, r := range results {
		ids = append(ids, r.ID)
		scoreMap[r.ID] = r.Score
	}

	// 构建 IN 查询
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf(`
		SELECT id, user_id, project, type, content, summary, tags, metadata, importance, created_at, updated_at
		FROM memories WHERE id IN (%s)
		ORDER BY created_at DESC
	`, strings.Join(placeholders, ","))

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询记忆失败: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		var summary *string
		err := rows.Scan(&m.ID, &m.UserID, &m.Project, &m.Type, &m.Content,
			&summary, &m.Tags, &m.Metadata, &m.Importance, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("扫描记忆行失败: %w", err)
		}
		if summary != nil {
			m.Summary = *summary
		}
		m.Score = scoreMap[m.ID]
		memories = append(memories, m)
	}
	return memories, nil
}

// GetContext 获取项目上下文（换设备时自动恢复）
func (s *Service) GetContext(ctx context.Context, userID string, req ContextRequest) (*ContextResponse, error) {
	resp := &ContextResponse{Project: req.Project}

	// 获取项目最新状态
	err := s.db.Pool.QueryRow(ctx, `
		SELECT last_device, last_branch, last_activity, updated_at
		FROM project_status WHERE project = $1 AND user_id = $2
	`, req.Project, userID).Scan(&resp.LastDevice, &resp.LastBranch, &resp.LastActivity, new(time.Time))
	if err != nil {
		// 没有状态记录，不报错
		log.Printf("获取项目状态: %v", err)
	}

	// 获取最近的记忆
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, user_id, project, type, content, summary, tags, metadata, importance, created_at, updated_at
		FROM memories
		WHERE project = $1 AND user_id = $2
		ORDER BY created_at DESC
		LIMIT 20
	`, req.Project, userID)
	if err != nil {
		return nil, fmt.Errorf("查询最近记忆失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m Memory
		var summary *string
		err := rows.Scan(&m.ID, &m.UserID, &m.Project, &m.Type, &m.Content,
			&summary, &m.Tags, &m.Metadata, &m.Importance, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			continue
		}
		if summary != nil {
			m.Summary = *summary
		}
		resp.RecentMemories = append(resp.RecentMemories, m)
	}

	return resp, nil
}

// SyncSession 同步会话到云端
func (s *Service) SyncSession(ctx context.Context, userID string, req SessionSyncRequest) error {
	id := uuid.New().String()

	// 保存会话
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, project, session_id, device, branch, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, userID, req.Project, req.SessionID, req.Device, req.Branch, req.Content, time.Now())
	if err != nil {
		return fmt.Errorf("保存会话失败: %w", err)
	}

	// 更新项目状态（复合主键：user_id + project）
	_, err = s.db.Pool.Exec(ctx, `
		INSERT INTO project_status (user_id, project, last_device, last_branch, last_activity, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, project) DO UPDATE SET
			last_device = EXCLUDED.last_device,
			last_branch = EXCLUDED.last_branch,
			last_activity = EXCLUDED.last_activity,
			updated_at = EXCLUDED.updated_at
	`, userID, req.Project, req.Device, req.Branch, "session synced", time.Now())
	if err != nil {
		return fmt.Errorf("更新项目状态失败: %w", err)
	}

	return nil
}

// ListProjects 列出所有项目
func (s *Service) ListProjects(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT DISTINCT project FROM memories WHERE user_id = $1 ORDER BY project
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("查询项目列表失败: %w", err)
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

// StatsResponse 统计响应
type StatsResponse struct {
	Memories       int      `json:"memories"`
	Projects       int      `json:"projects"`
	Sessions       int      `json:"sessions"`
	ProjectList    []string `json:"project_list"`
	RecentMemories []Memory `json:"recent_memories"`
}

// GetStats 获取统计数据
func (s *Service) GetStats(ctx context.Context, userID string) (*StatsResponse, error) {
	resp := &StatsResponse{}

	// 记忆总数
	s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM memories WHERE user_id = $1`, userID).Scan(&resp.Memories)

	// 项目数
	s.db.Pool.QueryRow(ctx, `SELECT COUNT(DISTINCT project) FROM memories WHERE user_id = $1`, userID).Scan(&resp.Projects)

	// 会话数
	s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, userID).Scan(&resp.Sessions)

	// 项目列表
	resp.ProjectList, _ = s.ListProjects(ctx, userID)

	// 最近 10 条记忆
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, user_id, project, type, content, summary, tags, metadata, importance, created_at, updated_at
		FROM memories WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 10
	`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m Memory
			var summary *string
			err := rows.Scan(&m.ID, &m.UserID, &m.Project, &m.Type, &m.Content,
				&summary, &m.Tags, &m.Metadata, &m.Importance, &m.CreatedAt, &m.UpdatedAt)
			if err == nil {
				if summary != nil {
					m.Summary = *summary
				}
				resp.RecentMemories = append(resp.RecentMemories, m)
			}
		}
	}

	return resp, nil
}

// Session 会话记录
type Session struct {
	ID          string    `json:"id"`
	Project     string    `json:"project"`
	SessionID   string    `json:"session_id"`
	Device      string    `json:"device"`
	Branch      string    `json:"branch"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListSessions 列出会话
func (s *Service) ListSessions(ctx context.Context, userID string) ([]Session, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, project, session_id, COALESCE(device,''), COALESCE(branch,''), created_at
		FROM sessions WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 50
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.Project, &s.SessionID, &s.Device, &s.Branch, &s.CreatedAt); err == nil {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

// ProjectDetail 项目详情
type ProjectDetail struct {
	Name          string    `json:"name"`
	MemoryCount   int       `json:"memory_count"`
	SessionCount  int       `json:"session_count"`
	LastDevice    string    `json:"last_device,omitempty"`
	LastBranch    string    `json:"last_branch,omitempty"`
	LastActivity  string    `json:"last_activity,omitempty"`
	LastUpdated   time.Time `json:"last_updated,omitempty"`
}

// GetProjectDetail 获取项目详情
func (s *Service) GetProjectDetail(ctx context.Context, userID, project string) (*ProjectDetail, error) {
	d := &ProjectDetail{Name: project}
	s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM memories WHERE user_id=$1 AND project=$2`, userID, project).Scan(&d.MemoryCount)
	s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE user_id=$1 AND project=$2`, userID, project).Scan(&d.SessionCount)

	var lastDevice, lastBranch, lastActivity *string
	var lastUpdated *time.Time
	err := s.db.Pool.QueryRow(ctx, `
		SELECT last_device, last_branch, last_activity, updated_at
		FROM project_status WHERE user_id=$1 AND project=$2
	`, userID, project).Scan(&lastDevice, &lastBranch, &lastActivity, &lastUpdated)
	if err == nil {
		if lastDevice != nil { d.LastDevice = *lastDevice }
		if lastBranch != nil { d.LastBranch = *lastBranch }
		if lastActivity != nil { d.LastActivity = *lastActivity }
		if lastUpdated != nil { d.LastUpdated = *lastUpdated }
	}
	return d, nil
}

// ListMemoriesByProject 按项目列出记忆
func (s *Service) ListMemoriesByProject(ctx context.Context, userID, project string, memType string, limit int) ([]Memory, error) {
	if limit <= 0 { limit = 50 }
	query := `SELECT id, user_id, project, type, content, summary, tags, metadata, importance, created_at, updated_at
		FROM memories WHERE user_id=$1 AND project=$2`
	args := []any{userID, project}
	if memType != "" {
		query += ` AND type=$3`
		args = append(args, memType)
	}
	query += ` ORDER BY created_at DESC LIMIT ` + fmt.Sprintf("%d", limit)

	rows, err := s.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询记忆失败: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		var summary *string
		if err := rows.Scan(&m.ID, &m.UserID, &m.Project, &m.Type, &m.Content, &summary, &m.Tags, &m.Metadata, &m.Importance, &m.CreatedAt, &m.UpdatedAt); err == nil {
			if summary != nil { m.Summary = *summary }
			memories = append(memories, m)
		}
	}
	return memories, nil
}

// ListSessionsByProject 按项目列出会话
func (s *Service) ListSessionsByProject(ctx context.Context, userID, project string) ([]SessionFull, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, project, session_id, COALESCE(device,''), COALESCE(branch,''), content, created_at
		FROM sessions WHERE user_id=$1 AND project=$2
		ORDER BY created_at DESC LIMIT 50
	`, userID, project)
	if err != nil {
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}
	defer rows.Close()

	var sessions []SessionFull
	for rows.Next() {
		var s SessionFull
		if err := rows.Scan(&s.ID, &s.Project, &s.SessionID, &s.Device, &s.Branch, &s.Content, &s.CreatedAt); err == nil {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

// SessionFull 完整会话（含内容）
type SessionFull struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	SessionID string    `json:"session_id"`
	Device    string    `json:"device"`
	Branch    string    `json:"branch"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// UpdateMemory 更新记忆
func (s *Service) UpdateMemory(ctx context.Context, userID, memoryID string, content, memType string, tags []string) error {
	_, err := s.db.Pool.Exec(ctx, `
		UPDATE memories SET content=$1, type=$2, tags=$3, updated_at=$4
		WHERE id=$5 AND user_id=$6
	`, content, memType, tags, time.Now(), memoryID, userID)
	if err != nil {
		return fmt.Errorf("更新记忆失败: %w", err)
	}

	// 更新 Qdrant 向量
	go func() {
		vec, err := s.embedder.Embed(context.Background(), content)
		if err != nil {
			log.Printf("更新 embedding 失败: %v", err)
			return
		}
		payload := map[string]any{"memory_id": memoryID, "user_id": userID, "type": memType, "tags": tags}
		s.qdrant.Upsert(context.Background(), memoryID, vec, payload)
	}()
	return nil
}

// ConfigFile 配置文件
type ConfigFile struct {
	FileKey   string    `json:"file_key"`
	Content   string    `json:"content"`
	Device    string    `json:"device"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConfigPush 推送配置文件到服务器
func (s *Service) ConfigPush(ctx context.Context, userID string, fileKey, content, device string) error {
	now := time.Now()

	// 保存历史版本
	s.db.Pool.Exec(ctx, `
		INSERT INTO config_history (user_id, file_key, content, device, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, fileKey, content, device, now)

	// 清理旧历史（保留最近 20 条）
	go func() {
		s.db.Pool.Exec(context.Background(), `
			DELETE FROM config_history WHERE id IN (
				SELECT id FROM config_history WHERE user_id=$1 AND file_key=$2
				ORDER BY created_at DESC OFFSET 20
			)
		`, userID, fileKey)
	}()

	// upsert 当前版本
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO configs (user_id, file_key, content, device, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, file_key) DO UPDATE SET
			content = EXCLUDED.content,
			device = EXCLUDED.device,
			updated_at = EXCLUDED.updated_at
	`, userID, fileKey, content, device, now)
	return err
}

// ConfigPull 从服务器拉取配置文件
func (s *Service) ConfigPull(ctx context.Context, userID, fileKey string) (*ConfigFile, error) {
	var cfg ConfigFile
	err := s.db.Pool.QueryRow(ctx, `
		SELECT file_key, content, COALESCE(device,''), updated_at
		FROM configs WHERE user_id=$1 AND file_key=$2
	`, userID, fileKey).Scan(&cfg.FileKey, &cfg.Content, &cfg.Device, &cfg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("配置不存在: %s", fileKey)
	}
	return &cfg, nil
}

// ConfigList 列出所有配置文件
func (s *Service) ConfigList(ctx context.Context, userID string) ([]ConfigFile, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT file_key, COALESCE(device,''), updated_at
		FROM configs WHERE user_id=$1 ORDER BY file_key
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []ConfigFile
	for rows.Next() {
		var c ConfigFile
		if err := rows.Scan(&c.FileKey, &c.Device, &c.UpdatedAt); err == nil {
			configs = append(configs, c)
		}
	}
	return configs, nil
}

// ========== 团队功能 ==========

// CreateTeam 创建团队
func (s *Service) CreateTeam(ctx context.Context, name, ownerID string) (*Team, error) {
	id := uuid.New().String()
	now := time.Now()
	_, err := s.db.Pool.Exec(ctx, `INSERT INTO teams (id, name, owner_id, created_at) VALUES ($1,$2,$3,$4)`, id, name, ownerID, now)
	if err != nil {
		return nil, fmt.Errorf("创建团队失败: %w", err)
	}
	// 自动加入 owner
	s.db.Pool.Exec(ctx, `INSERT INTO team_members (team_id, user_id, role, joined_at) VALUES ($1,$2,'owner',$3)`, id, ownerID, now)
	return &Team{ID: id, Name: name, OwnerID: ownerID, CreatedAt: now}, nil
}

// ListTeams 列出用户的团队
func (s *Service) ListTeams(ctx context.Context, userID string) ([]Team, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT t.id, t.name, t.owner_id, t.created_at FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = $1 ORDER BY t.name
	`, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var teams []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.OwnerID, &t.CreatedAt); err == nil {
			teams = append(teams, t)
		}
	}
	return teams, nil
}

// AddTeamMember 添加团队成员
func (s *Service) AddTeamMember(ctx context.Context, teamID, userID, role string) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO team_members (team_id, user_id, role, joined_at) VALUES ($1,$2,$3,$4)
		ON CONFLICT (team_id, user_id) DO UPDATE SET role=EXCLUDED.role
	`, teamID, userID, role, time.Now())
	return err
}

// ListTeamMembers 列出团队成员
func (s *Service) ListTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT tm.team_id, tm.user_id, u.username, tm.role, tm.joined_at
		FROM team_members tm JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = $1 ORDER BY tm.joined_at
	`, teamID)
	if err != nil { return nil, err }
	defer rows.Close()
	var members []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err == nil {
			members = append(members, m)
		}
	}
	return members, nil
}

// RemoveTeamMember 移除团队成员
func (s *Service) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	_, err := s.db.Pool.Exec(ctx, `DELETE FROM team_members WHERE team_id=$1 AND user_id=$2 AND role!='owner'`, teamID, userID)
	return err
}

// ShareMemory 共享记忆到团队
func (s *Service) ShareMemory(ctx context.Context, teamID, memoryID, sharedBy string) error {
	_, err := s.db.Pool.Exec(ctx, `
		INSERT INTO shared_memories (team_id, memory_id, shared_by, shared_at) VALUES ($1,$2,$3,$4)
		ON CONFLICT (team_id, memory_id) DO NOTHING
	`, teamID, memoryID, sharedBy, time.Now())
	return err
}

// ListSharedMemories 列出团队共享记忆
func (s *Service) ListSharedMemories(ctx context.Context, teamID string, limit int) ([]Memory, error) {
	if limit <= 0 { limit = 50 }
	rows, err := s.db.Pool.Query(ctx, `
		SELECT m.id, m.user_id, m.project, m.type, m.content, m.summary, m.tags, m.metadata, m.importance, m.created_at, m.updated_at
		FROM memories m JOIN shared_memories sm ON m.id = sm.memory_id
		WHERE sm.team_id = $1 ORDER BY sm.shared_at DESC LIMIT $2
	`, teamID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var memories []Memory
	for rows.Next() {
		var m Memory
		var summary *string
		if err := rows.Scan(&m.ID, &m.UserID, &m.Project, &m.Type, &m.Content, &summary, &m.Tags, &m.Metadata, &m.Importance, &m.CreatedAt, &m.UpdatedAt); err == nil {
			if summary != nil { m.Summary = *summary }
			memories = append(memories, m)
		}
	}
	return memories, nil
}

// ========== 每日简报 ==========

// GenerateBriefing 生成项目简报
func (s *Service) GenerateBriefing(ctx context.Context, userID, project string) (*Briefing, error) {
	// 获取最近 24 小时的记忆
	rows, err := s.db.Pool.Query(ctx, `
		SELECT type, content, importance, created_at FROM memories
		WHERE user_id=$1 AND ($2='' OR project=$2) AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY importance DESC, created_at DESC LIMIT 30
	`, userID, project)
	if err != nil { return nil, err }
	defer rows.Close()

	var items []string
	for rows.Next() {
		var mtype, content string
		var imp float64
		var cat time.Time
		if err := rows.Scan(&mtype, &content, &imp, &cat); err == nil {
			items = append(items, fmt.Sprintf("[%s][%.1f] %s", mtype, imp, content[:min(len(content), 200)]))
		}
	}

	if len(items) == 0 {
		return &Briefing{Content: "No activity in the last 24 hours.", Period: "daily"}, nil
	}

	// 简单生成（不依赖 LLM，直接结构化）
	content := fmt.Sprintf("# Daily Briefing — %s\n\n", time.Now().Format("2006-01-02"))
	if project != "" {
		content += fmt.Sprintf("Project: %s\n\n", project)
	}
	content += fmt.Sprintf("## Summary\n- %d memories in last 24h\n\n## Top Items\n", len(items))
	for i, item := range items {
		if i >= 10 { break }
		content += fmt.Sprintf("- %s\n", item)
	}

	// 保存简报
	b := &Briefing{
		ID:        uuid.New().String(),
		Project:   project,
		Content:   content,
		Period:    "daily",
		CreatedAt: time.Now(),
	}
	s.db.Pool.Exec(ctx, `INSERT INTO briefings (id, user_id, project, content, period, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		b.ID, userID, project, content, "daily", b.CreatedAt)
	return b, nil
}

func min(a, b int) int { if a < b { return a }; return b }

// Delete 删除记忆
func (s *Service) Delete(ctx context.Context, userID string, memoryID string) error {
	_, err := s.db.Pool.Exec(ctx, `
		DELETE FROM memories WHERE id = $1 AND user_id = $2
	`, memoryID, userID)
	if err != nil {
		return fmt.Errorf("删除记忆失败: %w", err)
	}

	// 同步删除 Qdrant 中的向量
	go func() {
		if err := s.qdrant.Delete(context.Background(), []string{memoryID}); err != nil {
			log.Printf("删除 Qdrant 向量失败: %v", err)
		}
	}()

	return nil
}
