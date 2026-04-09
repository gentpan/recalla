-- 团队项目绑定表
CREATE TABLE IF NOT EXISTS team_projects (
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    project     VARCHAR(200) NOT NULL,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, project)
);

-- 团队邀请表
CREATE TABLE IF NOT EXISTS team_invites (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    invited_by  UUID NOT NULL REFERENCES users(id),
    username    VARCHAR(100) NOT NULL,  -- 被邀请的用户名
    status      VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, accepted, rejected
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_team_invites_user ON team_invites(username, status);

-- 团队活动日志
CREATE TABLE IF NOT EXISTS team_activity (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id     UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,
    action      VARCHAR(100) NOT NULL,  -- memory_shared, member_added, project_added, etc.
    detail      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_team_activity_team ON team_activity(team_id, created_at DESC);
