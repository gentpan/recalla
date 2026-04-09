-- 配置文件同步表
CREATE TABLE IF NOT EXISTS configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
    file_key    VARCHAR(200) NOT NULL,  -- 标识：claude-md, claude-json-mcp, cursor-rules, codex-instructions
    content     TEXT NOT NULL,
    device      VARCHAR(200),           -- 最后修改的设备
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, file_key)
);

-- 配置历史表（保留最近 20 个版本）
CREATE TABLE IF NOT EXISTS config_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
    file_key    VARCHAR(200) NOT NULL,
    content     TEXT NOT NULL,
    device      VARCHAR(200),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_config_history_key ON config_history(user_id, file_key, created_at DESC);
