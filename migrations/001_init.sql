-- Recalla 数据库初始化
-- 记忆表

CREATE TABLE IF NOT EXISTS memories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
    project     VARCHAR(200) NOT NULL,
    type        VARCHAR(50) NOT NULL DEFAULT 'note',
    content     TEXT NOT NULL,
    summary     TEXT,
    tags        TEXT[] DEFAULT '{}',
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
CREATE INDEX IF NOT EXISTS idx_memories_project ON memories(project);
CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
CREATE INDEX IF NOT EXISTS idx_memories_tags ON memories USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memories_metadata ON memories USING GIN(metadata);

-- 会话表（记录每次 AI 会话，用于压缩）
CREATE TABLE IF NOT EXISTS sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
    project     VARCHAR(200) NOT NULL,
    session_id  VARCHAR(200) NOT NULL,
    device      VARCHAR(200),
    branch      VARCHAR(200),
    content     TEXT NOT NULL,
    compressed  TEXT,
    is_compressed BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project);
CREATE INDEX IF NOT EXISTS idx_sessions_user_project ON sessions(user_id, project);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at DESC);

-- 项目状态表（记录每个项目的最新状态）
-- 使用 user_id + project 做唯一约束，支持多用户
CREATE TABLE IF NOT EXISTS project_status (
    user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
    project     VARCHAR(200) NOT NULL,
    last_device VARCHAR(200),
    last_branch VARCHAR(200),
    last_activity TEXT,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, project)
);
