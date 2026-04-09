-- 知识图谱：实体事实表（带时间有效性）
CREATE TABLE IF NOT EXISTS entity_facts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
    subject     VARCHAR(500) NOT NULL,    -- 主体：人/项目/技术
    predicate   VARCHAR(200) NOT NULL,    -- 关系：works_on, uses, decided, owns
    object      VARCHAR(500) NOT NULL,    -- 客体：项目/技术/方案
    project     VARCHAR(200),
    valid_from  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,              -- NULL = 当前有效
    source_id   UUID,                     -- 来源记忆 ID
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_entity_facts_subject ON entity_facts(user_id, subject);
CREATE INDEX IF NOT EXISTS idx_entity_facts_object ON entity_facts(user_id, object);
CREATE INDEX IF NOT EXISTS idx_entity_facts_project ON entity_facts(user_id, project);
CREATE INDEX IF NOT EXISTS idx_entity_facts_valid ON entity_facts(user_id, valid_until);

-- 记忆层级（Wing → Room）
ALTER TABLE memories ADD COLUMN IF NOT EXISTS wing VARCHAR(200);  -- 顶层分类（项目名/人名）
ALTER TABLE memories ADD COLUMN IF NOT EXISTS room VARCHAR(200);  -- 主题分类（auth/database/deploy）
