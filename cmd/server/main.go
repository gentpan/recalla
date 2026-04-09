package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gentpan/recalla/internal/api"
	authpkg "github.com/gentpan/recalla/internal/auth"
	tgpkg "github.com/gentpan/recalla/internal/telegram"
	"github.com/gentpan/recalla/internal/compress"
	"github.com/gentpan/recalla/internal/config"
	"github.com/gentpan/recalla/internal/db"
	"github.com/gentpan/recalla/internal/mcp"
	"github.com/gentpan/recalla/internal/memory"
	"github.com/gentpan/recalla/internal/vector"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Recalla 服务启动中...")

	cfg := config.Load()
	ctx := context.Background()

	// 连接数据库
	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Close()
	log.Println("数据库连接成功")

	// 执行迁移
	if err := database.Migrate(ctx); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}
	log.Println("数据库迁移完成")

	// 初始化认证服务
	authSvc := authpkg.NewService(database)
	if err := authSvc.EnsureAdmin(ctx); err != nil {
		log.Printf("警告: 创建默认管理员失败: %v", err)
	}

	// 初始化 Qdrant
	qdrant := vector.NewQdrant(cfg.QdrantURL, cfg.QdrantCollection)
	if err := qdrant.EnsureCollection(ctx, cfg.EmbeddingDim); err != nil {
		log.Printf("警告: Qdrant 初始化失败（语义搜索不可用）: %v", err)
	} else {
		log.Println("Qdrant 连接成功")
	}

	// 初始化 Embedder（动态从 Config 获取当前 active provider）
	embedder := vector.NewEmbedder(cfg)

	// 初始化压缩器（动态从 Config 获取当前 active provider）
	compressor := compress.NewCompressor(cfg)

	// 初始化 Memory 服务
	memService := memory.NewService(database, qdrant, embedder)

	// 初始化 HTTP 路由
	mux := http.NewServeMux()

	// 注册 REST API
	handler := api.NewHandler(memService, compressor, cfg, authSvc)
	handler.Register(mux)

	// 注册 MCP 端点
	mcpServer := mcp.NewServer(memService, compressor)
	mux.Handle("/mcp", mcpServer)

	// Telegram Bot Webhook
	if cfg.TelegramToken != "" {
		tgBot := tgpkg.NewBot(cfg.TelegramToken, memService, compressor)
		mux.HandleFunc("POST /api/telegram/webhook", tgBot.HandleWebhook)
		log.Println("Telegram Bot 已启用")
	}

	// 静态文件服务（Dashboard）
	mux.Handle("/", http.FileServer(http.Dir("web/static")))

	// 组装中间件
	var h http.Handler = mux
	h = api.CORSMiddleware(h)
	h = api.AuthMiddleware(authSvc, h)

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("收到关闭信号，正在关闭服务...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	log.Printf("Recalla 服务已启动: http://0.0.0.0%s", cfg.Addr)
	log.Printf("  Dashboard: http://0.0.0.0%s/", cfg.Addr)
	log.Printf("  REST API:  http://0.0.0.0%s/api/", cfg.Addr)
	log.Printf("  MCP 端点:  http://0.0.0.0%s/mcp", cfg.Addr)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("服务异常退出: %v", err)
	}
	log.Println("Recalla 服务已关闭")
}
