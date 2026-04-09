## Recalla 记忆规则（必须执行）

### 对话开始时
每次对话开始时，**必须**先调用 `context_restore` 获取项目上下文：
```json
{"project": "当前项目名"}
```
根据返回的上下文了解：上次在哪台设备、哪个分支、做到哪了。

### 开始新任务前
调用 `memory_search` 搜索相关记忆，避免重复工作：
```json
{"query": "要做的任务描述", "project": "当前项目名"}
```

### 做出重要决策时
调用 `memory_save` 记录以下内容：
- 架构决策（选了什么方案、为什么）
- 代码变更（改了什么、为什么）
- Bug 修复（问题和解决方案）
- 部署操作（部署到哪、配置了什么）

```json
{
  "project": "当前项目名",
  "type": "decision",
  "content": "决策内容",
  "tags": ["相关标签"],
  "metadata": {"branch": "当前分支", "device": "设备名"}
}
```

### 对话结束时
调用 `session_sync` 同步本次会话：
```json
{
  "project": "当前项目名",
  "session_id": "唯一会话ID",
  "content": "本次会话摘要：做了什么、结论是什么、下一步计划",
  "device": "当前设备名",
  "branch": "当前git分支"
}
```

### 会话过长时
当会话内容过长、接近上下文限制时，调用 `session_compress` 压缩：
```json
{"content": "需要压缩的会话内容"}
```
将压缩结果保存为新的记忆。
