-- Phase 2: 重要性评分 + 标准标签 + 团队 + 简报

-- 1. memories 表加重要性评分
ALTER TABLE memories ADD COLUMN IF NOT EXISTS importance REAL DEFAULT 0.5;

-- 2. 标准标签枚举（不用 enum，用约定）
-- insight, decision, fact, procedure, experience, code, bug, deploy

-- 3. 团队表
CREATE TABLE IF NOT EXISTS teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(200) UNIQUE NOT NULL,
    owner_id    UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. 团队成员表
CREATE TABLE IF NOT EXISTS team_members (
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        VARCHAR(20) NOT NULL DEFAULT 'member',  -- owner, admin, member
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

-- 5. 共享记忆表（团队级别的记忆）
CREATE TABLE IF NOT EXISTS shared_memories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    memory_id   UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    shared_by   UUID NOT NULL REFERENCES users(id),
    shared_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(team_id, memory_id)
);

CREATE INDEX IF NOT EXISTS idx_shared_memories_team ON shared_memories(team_id);

-- 6. 简报表
CREATE TABLE IF NOT EXISTS briefings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
    project     VARCHAR(200),
    content     TEXT NOT NULL,
    period      VARCHAR(20) NOT NULL DEFAULT 'daily',  -- daily, weekly
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_briefings_user ON briefings(user_id, created_at DESC);
