package mcp

// getToolDefinitions 返回 MCP 工具定义
func getToolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "memory_save",
			"description": "保存一条记忆到 Recalla 云端。用于记录重要的代码决策、架构变更、调试过程、部署信息等。AI 应在做出重要决策或完成关键操作后自动调用此工具。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project": map[string]any{
						"type":        "string",
						"description": "项目名称，例如 zhanxing.io、bluecdn 等",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "记忆类型：conversation（对话）、code（代码）、decision（决策）、note（笔记）、session（会话）",
						"enum":        []string{"conversation", "code", "decision", "note", "session"},
					},
					"content": map[string]any{
						"type":        "string",
						"description": "记忆内容，尽量结构化，包含关键信息",
					},
					"tags": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "标签列表，用于分类和过滤",
					},
					"metadata": map[string]any{
						"type":        "object",
						"description": "额外元数据，如 git 分支、设备名、文件路径等",
					},
				},
				"required": []string{"project", "content"},
			},
		},
		{
			"name":        "memory_search",
			"description": "语义搜索 Recalla 中的记忆。根据自然语言查询找到最相关的记忆。AI 应在开始新任务前搜索相关记忆，避免重复工作。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索查询，自然语言描述你要找的内容",
					},
					"project": map[string]any{
						"type":        "string",
						"description": "限制搜索范围到指定项目",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "限制搜索的记忆类型",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回结果数量，默认 10",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "context_restore",
			"description": "恢复项目上下文。获取指定项目的最新状态（上次使用的设备、分支、最近的记忆等）。AI 应在每次对话开始时调用此工具，了解用户上次做到哪里。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project": map[string]any{
						"type":        "string",
						"description": "项目名称",
					},
					"device": map[string]any{
						"type":        "string",
						"description": "当前设备名称",
					},
				},
				"required": []string{"project"},
			},
		},
		{
			"name":        "session_sync",
			"description": "将当前 AI 会话同步到 Recalla 云端。记录会话内容、设备信息和 git 分支。AI 应在对话结束或重要节点时调用。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"project": map[string]any{
						"type":        "string",
						"description": "项目名称",
					},
					"session_id": map[string]any{
						"type":        "string",
						"description": "会话 ID",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "会话内容摘要",
					},
					"device": map[string]any{
						"type":        "string",
						"description": "当前设备名称",
					},
					"branch": map[string]any{
						"type":        "string",
						"description": "当前 git 分支",
					},
				},
				"required": []string{"project", "session_id", "content", "device"},
			},
		},
		{
			"name":        "session_compress",
			"description": "使用 AI 压缩会话内容，提取关键信息。适用于长会话的摘要生成。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{
						"type":        "string",
						"description": "需要压缩的会话内容",
					},
				},
				"required": []string{"content"},
			},
		},
		{
			"name":        "project_list",
			"description": "列出所有已记录的项目。",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}
