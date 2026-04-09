-- 修复 project_status 表：改为复合主键 (user_id, project)
-- 先删除旧表再重建（数据量极小，可以接受）
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'project_status') THEN
        -- 检查是否已经是复合主键
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.key_column_usage
            WHERE table_name = 'project_status' AND column_name = 'user_id'
            AND constraint_name IN (SELECT constraint_name FROM information_schema.table_constraints WHERE table_name = 'project_status' AND constraint_type = 'PRIMARY KEY')
        ) THEN
            DROP TABLE project_status;
            CREATE TABLE project_status (
                user_id     VARCHAR(100) NOT NULL DEFAULT 'default',
                project     VARCHAR(200) NOT NULL,
                last_device VARCHAR(200),
                last_branch VARCHAR(200),
                last_activity TEXT,
                updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                PRIMARY KEY (user_id, project)
            );
        END IF;
    END IF;
END $$;

-- 添加 sessions 的索引（如果不存在）
CREATE INDEX IF NOT EXISTS idx_sessions_user_project ON sessions(user_id, project);
