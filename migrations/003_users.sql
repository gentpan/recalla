-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    VARCHAR(100) UNIQUE NOT NULL,
    password    VARCHAR(200) NOT NULL,  -- bcrypt hash
    role        VARCHAR(20) NOT NULL DEFAULT 'user',  -- admin / user
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- API Key 表
CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key         VARCHAR(100) UNIQUE NOT NULL,
    name        VARCHAR(200) NOT NULL DEFAULT 'default',
    last_used   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_key ON api_keys(key);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);

-- 如果没有任何用户，创建默认管理员 (admin / admin123)
-- 密码 bcrypt hash: $2a$10$... 由代码初始化时插入
