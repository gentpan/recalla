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
		{
			"name":        "config_push",
			"description": "推送本地配置文件到 Recalla 服务器。用于同步 CLAUDE.md、.cursorrules 等 AI 工具配置文件到云端，确保多台设备配置一致。当配置文件被修改后应自动调用。支持的 file_key：claude-md（~/.claude/CLAUDE.md）、cursor-rules（.cursor/rules/）、codex-instructions（~/.codex/instructions.md）。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_key": map[string]any{
						"type":        "string",
						"description": "配置文件标识：claude-md, cursor-rules, codex-instructions",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "配置文件的完整内容",
					},
					"device": map[string]any{
						"type":        "string",
						"description": "当前设备名称",
					},
				},
				"required": []string{"file_key", "content"},
			},
		},
		{
			"name":        "config_pull",
			"description": "从 Recalla 服务器拉取最新的配置文件。用于在新设备上获取最新的 CLAUDE.md 等配置。在对话开始时，如果检测到本地配置可能过期，应调用此工具获取最新版本。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_key": map[string]any{
						"type":        "string",
						"description": "配置文件标识：claude-md, cursor-rules, codex-instructions",
					},
				},
				"required": []string{"file_key"},
			},
		},
		{
			"name":        "team_search",
			"description": "搜索团队共享记忆。在团队协作场景中，搜索所有团队成员的记忆。需要提供 team_id，可通过 project_list 或 Dashboard 获取。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"team_id": map[string]any{
						"type":        "string",
						"description": "团队 ID",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "搜索查询",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回结果数量，默认 10",
					},
				},
				"required": []string{"team_id", "query"},
			},
		},
		{
			"name":        "team_share",
			"description": "将一条记忆共享到团队。团队其他成员可以通过 team_search 搜索到此记忆。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"team_id": map[string]any{
						"type":        "string",
						"description": "团队 ID",
					},
					"memory_id": map[string]any{
						"type":        "string",
						"description": "要共享的记忆 ID",
					},
				},
				"required": []string{"team_id", "memory_id"},
			},
		},
		{
			"name":        "add_fact",
			"description": "添加一条实体事实到知识图谱。用于记录谁负责什么、项目用了什么技术等事实性信息。支持时间有效性（valid_until）。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"subject": map[string]any{
						"type":        "string",
						"description": "主体：人名、项目名、技术名",
					},
					"predicate": map[string]any{
						"type":        "string",
						"description": "关系：works_on, uses, decided, owns, manages, deployed_to",
					},
					"object": map[string]any{
						"type":        "string",
						"description": "客体：项目、技术、方案、服务器",
					},
					"project": map[string]any{
						"type":        "string",
						"description": "关联项目",
					},
				},
				"required": []string{"subject", "predicate", "object"},
			},
		},
		{
			"name":        "query_facts",
			"description": "查询知识图谱中的事实。例如：查询某人负责什么项目、某项目用了什么技术。只返回当前有效的事实。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"subject": map[string]any{
						"type":        "string",
						"description": "按主体过滤（模糊匹配）",
					},
					"predicate": map[string]any{
						"type":        "string",
						"description": "按关系过滤",
					},
					"project": map[string]any{
						"type":        "string",
						"description": "按项目过滤",
					},
				},
			},
		},
	}
}
