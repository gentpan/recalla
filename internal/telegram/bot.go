package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gentpan/recalla/internal/compress"
	"github.com/gentpan/recalla/internal/memory"
)

// Bot Telegram 机器人
type Bot struct {
	token      string
	mem        *memory.Service
	compressor *compress.Compressor
	client     *http.Client
	// 用户当前项目（简单状态管理）
	userProject map[string]string // chat_id -> project
}

// NewBot 创建 Telegram Bot
func NewBot(token string, mem *memory.Service, compressor *compress.Compressor) *Bot {
	return &Bot{
		token:       token,
		mem:         mem,
		compressor:  compressor,
		client:      &http.Client{},
		userProject: make(map[string]string),
	}
}

// Update Telegram 更新消息
type Update struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		From      *struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"from"`
		Chat *struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// HandleWebhook 处理 Telegram Webhook
func (b *Bot) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var update Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	if update.Message == nil || update.Message.Text == "" {
		w.WriteHeader(200)
		return
	}

	chatID := update.Message.Chat.ID
	text := strings.TrimSpace(update.Message.Text)
	userID := "default" // TODO: 关联到 Recalla 用户

	log.Printf("[Telegram] chat=%d text=%s", chatID, text[:minInt(len(text), 50)])

	// 解析命令
	switch {
	case text == "/start" || text == "/help":
		b.sendHelp(chatID)

	case strings.HasPrefix(text, "/search "):
		query := strings.TrimPrefix(text, "/search ")
		b.handleSearch(chatID, userID, query)

	case strings.HasPrefix(text, "/s "):
		query := strings.TrimPrefix(text, "/s ")
		b.handleSearch(chatID, userID, query)

	case strings.HasPrefix(text, "/save "):
		content := strings.TrimPrefix(text, "/save ")
		b.handleSave(chatID, userID, content)

	case strings.HasPrefix(text, "/context"):
		project := strings.TrimSpace(strings.TrimPrefix(text, "/context"))
		b.handleContext(chatID, userID, project)

	case strings.HasPrefix(text, "/project "):
		project := strings.TrimPrefix(text, "/project ")
		b.userProject[fmt.Sprintf("%d", chatID)] = project
		b.send(chatID, fmt.Sprintf("Switched to project: *%s*", project))

	case text == "/projects":
		b.handleProjects(chatID, userID)

	case strings.HasPrefix(text, "/compress"):
		b.handleCompress(chatID, userID)

	case strings.HasPrefix(text, "/ask "):
		question := strings.TrimPrefix(text, "/ask ")
		b.handleAsk(chatID, userID, question)

	case text == "/briefing":
		b.handleBriefing(chatID, userID)

	default:
		// 非命令消息，忽略或自动保存
		if strings.HasPrefix(text, "/") {
			b.send(chatID, "Unknown command. Use /help")
		}
	}

	w.WriteHeader(200)
}

func (b *Bot) handleSearch(chatID int64, userID, query string) {
	project := b.userProject[fmt.Sprintf("%d", chatID)]
	memories, err := b.mem.Search(context.Background(), userID, memory.SearchRequest{
		Query: query, Project: project, Limit: 5,
	})
	if err != nil {
		b.send(chatID, "Search failed: "+err.Error())
		return
	}
	if len(memories) == 0 {
		b.send(chatID, "No memories found.")
		return
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("Found %d memories:\n\n", len(memories)))
	for i, m := range memories {
		content := m.Content
		if len(content) > 150 { content = content[:150] + "..." }
		msg.WriteString(fmt.Sprintf("*%d.* [%s] `%s`\n%s\nScore: %.0f%% | %s\n\n",
			i+1, m.Type, m.Project, content, m.Score*100, m.CreatedAt.Format("2006-01-02 15:04")))
	}
	b.send(chatID, msg.String())
}

func (b *Bot) handleSave(chatID int64, userID, content string) {
	project := b.userProject[fmt.Sprintf("%d", chatID)]
	if project == "" { project = "general" }

	// 自动判断类型
	memType := "note"
	lower := strings.ToLower(content)
	if strings.Contains(lower, "决定") || strings.Contains(lower, "决策") || strings.Contains(lower, "decide") {
		memType = "decision"
	} else if strings.Contains(lower, "bug") || strings.Contains(lower, "fix") || strings.Contains(lower, "修复") {
		memType = "bug"
	} else if strings.Contains(lower, "deploy") || strings.Contains(lower, "部署") {
		memType = "deploy"
	}

	mem, err := b.mem.Save(context.Background(), userID, memory.SaveRequest{
		Project: project, Type: memType, Content: content,
		Tags: []string{"telegram"},
		Metadata: memory.Map{"source": "telegram", "chat_id": fmt.Sprintf("%d", chatID)},
	})
	if err != nil {
		b.send(chatID, "Save failed: "+err.Error())
		return
	}
	b.send(chatID, fmt.Sprintf("Saved to *%s* [%s]\nID: `%s`", project, memType, mem.ID[:8]))
}

func (b *Bot) handleContext(chatID int64, userID, project string) {
	if project == "" {
		project = b.userProject[fmt.Sprintf("%d", chatID)]
	}
	if project == "" {
		b.send(chatID, "Usage: /context <project>")
		return
	}

	ctx, err := b.mem.GetContext(context.Background(), userID, memory.ContextRequest{Project: project})
	if err != nil {
		b.send(chatID, "Failed: "+err.Error())
		return
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("*%s* context:\n\n", project))
	if ctx.LastDevice != "" { msg.WriteString(fmt.Sprintf("Device: `%s`\n", ctx.LastDevice)) }
	if ctx.LastBranch != "" { msg.WriteString(fmt.Sprintf("Branch: `%s`\n", ctx.LastBranch)) }
	msg.WriteString(fmt.Sprintf("\nRecent memories: %d\n", len(ctx.RecentMemories)))
	for i, m := range ctx.RecentMemories {
		if i >= 5 { break }
		content := m.Content
		if len(content) > 80 { content = content[:80] + "..." }
		msg.WriteString(fmt.Sprintf("- [%s] %s\n", m.Type, content))
	}
	b.send(chatID, msg.String())
}

func (b *Bot) handleProjects(chatID int64, userID string) {
	projects, err := b.mem.ListProjects(context.Background(), userID)
	if err != nil {
		b.send(chatID, "Failed: "+err.Error())
		return
	}
	if len(projects) == 0 {
		b.send(chatID, "No projects yet.")
		return
	}
	var msg strings.Builder
	msg.WriteString("Projects:\n\n")
	current := b.userProject[fmt.Sprintf("%d", chatID)]
	for _, p := range projects {
		marker := " "
		if p == current { marker = ">" }
		msg.WriteString(fmt.Sprintf("%s `%s`\n", marker, p))
	}
	msg.WriteString("\nUse /project <name> to switch")
	b.send(chatID, msg.String())
}

func (b *Bot) handleCompress(chatID int64, userID string) {
	b.send(chatID, "Compressing recent sessions...")
	// 获取最近记忆作为压缩内容
	stats, _ := b.mem.GetStats(context.Background(), userID)
	if stats == nil || len(stats.RecentMemories) == 0 {
		b.send(chatID, "No recent memories to compress.")
		return
	}
	var content strings.Builder
	for _, m := range stats.RecentMemories {
		content.WriteString(fmt.Sprintf("[%s] %s\n", m.Type, m.Content))
	}
	compressed, err := b.compressor.Compress(context.Background(), content.String())
	if err != nil {
		b.send(chatID, "Compression failed: "+err.Error())
		return
	}
	if len(compressed) > 4000 { compressed = compressed[:4000] + "..." }
	b.send(chatID, "Compressed:\n\n"+compressed)
}

func (b *Bot) handleBriefing(chatID int64, userID string) {
	project := b.userProject[fmt.Sprintf("%d", chatID)]
	briefing, err := b.mem.GenerateBriefing(context.Background(), userID, project)
	if err != nil {
		b.send(chatID, "Briefing failed: "+err.Error())
		return
	}
	b.send(chatID, briefing.Content)
}

func (b *Bot) handleAsk(chatID int64, userID, question string) {
	b.send(chatID, "Thinking...")

	// 搜索相关记忆
	project := b.userProject[fmt.Sprintf("%d", chatID)]
	memories, err := b.mem.Search(context.Background(), userID, memory.SearchRequest{
		Query: question, Project: project, Limit: 5,
	})
	if err != nil || len(memories) == 0 {
		b.send(chatID, "No relevant memories found for this question.")
		return
	}

	// 构建上下文
	var ctx strings.Builder
	for _, m := range memories {
		content := m.Content
		if len(content) > 300 { content = content[:300] }
		ctx.WriteString(fmt.Sprintf("[%s][%s] %s\n\n", m.Type, m.CreatedAt.Format("2006-01-02"), content))
	}

	// 用 LLM 回答
	prompt := fmt.Sprintf("Based on the following memories, answer the user's question concisely.\n\nMemories:\n%s\nQuestion: %s\n\nAnswer:", ctx.String(), question)
	answer, err := b.compressor.Compress(context.Background(), prompt)
	if err != nil {
		// fallback: 直接返回记忆
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("Related memories for: %s\n\n", question))
		for i, m := range memories {
			content := m.Content
			if len(content) > 150 { content = content[:150] + "..." }
			msg.WriteString(fmt.Sprintf("%d. [%s] %s\n%s\n\n", i+1, m.Type, m.CreatedAt.Format("01-02"), content))
		}
		b.send(chatID, msg.String())
		return
	}

	if len(answer) > 4000 { answer = answer[:4000] + "..." }
	b.send(chatID, answer)
}

func (b *Bot) sendHelp(chatID int64) {
	b.send(chatID, `*Recalla Bot*

/search <query> — Search memories
/s <query> — Search (shortcut)
/save <content> — Save a memory
/context <project> — Restore project context
/project <name> — Switch project
/projects — List all projects
/ask <question> — AI answers based on memories
/compress — Compress recent sessions
/briefing — Generate daily briefing
/help — Show this help`)
}

// Notify 向指定 chat 发送通知（供外部调用）
func (b *Bot) Notify(chatID int64, text string) {
	b.send(chatID, text)
}

// NotifyAll 向所有已知用户发送通知（简化版：用环境变量配置的 chat ID）
func (b *Bot) NotifyOwner(text string) {
	// 从环境变量获取 owner chat ID
	ownerChat := os.Getenv("RECALLA_TELEGRAM_CHAT_ID")
	if ownerChat == "" { return }
	var chatID int64
	fmt.Sscanf(ownerChat, "%d", &chatID)
	if chatID != 0 { b.send(chatID, text) }
}

// send 发送消息
func (b *Bot) send(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.token)
	body, _ := json.Marshal(map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	})
	resp, err := b.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[Telegram] send error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		log.Printf("[Telegram] send failed: %s", string(b))
	}
}

func minInt(a, b int) int { if a < b { return a }; return b }
