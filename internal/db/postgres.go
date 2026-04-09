package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB 数据库连接池
type DB struct {
	Pool *pgxpool.Pool
}

// New 创建数据库连接
func New(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("数据库 ping 失败: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Migrate 执行数据库迁移（按顺序执行所有迁移文件）
func (d *DB) Migrate(ctx context.Context) error {
	files := []string{"migrations/001_init.sql", "migrations/002_fix_project_status.sql", "migrations/003_users.sql"}
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", f, err)
		}
		_, err = d.Pool.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %w", f, err)
		}
	}
	return nil
}

// Close 关闭连接
func (d *DB) Close() {
	d.Pool.Close()
}
