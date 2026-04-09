# Recalla

**Switch devices, keep coding. Your AI remembers everything.**

Recalla is an open-source, self-hosted AI memory infrastructure. It solves the context loss problem when developers switch between devices — your AI knows what you were working on, which branch, what decisions were made, and continues seamlessly.

## The Problem

- You code on your MacBook, switch to your desktop — AI has no idea what you did
- Every new session starts from zero, you repeat yourself
- Wrong deployments, forgotten decisions, duplicated code

## How It Works

```
Any Device → AI Tool (Claude/Cursor/Codex) → MCP Protocol → Recalla Server
                                                                 ↓
                                              Postgres + Qdrant (Vector Search)
                                                                 ↓
                                              Memory stored, synced, searchable
```

Recalla sits between your AI tools and a persistent memory layer. It captures context, stores it on YOUR server, and brings it back when you need it — on any device.

## Features

- **Cross-device memory sync** — Save AI sessions, decisions, code context to your own server
- **Semantic search** — Find relevant memories using natural language (powered by vector search)
- **AI session compression** — Compress long conversations into structured summaries
- **Auto-tagging** — Memories automatically tagged based on content (decision/bug/deploy/code...)
- **Importance scoring** — Memories scored 0.1-1.0, decisions auto-scored higher
- **Knowledge graph** — Entity facts with temporal validity (who works on what, when)
- **Contradiction detection** — Automatically find conflicting memories
- **Team collaboration** — Create teams, invite members, share memories, team-wide search
- **Config sync** — Push CLAUDE.md and AI tool configs to server, new devices pull automatically
- **MCP protocol** — One server works with Claude Code, Cursor, VS Code, Codex (12 tools)
- **GitHub integration** — Auto-record pushes and PRs as memories via Webhook
- **Telegram bot** — Search, save, ask questions, get briefings from Telegram
- **CLI tool** — `recalla search/save/context/status` from terminal
- **Multi-provider AI** — Support OpenAI, Qwen, DeepSeek, Ollama, or any OpenAI-compatible API
- **Dashboard** — Web UI to manage memories, sessions, projects, teams, and settings
- **Daily briefing** — Generate project activity summaries
- **User auth** — Login system with API key management
- **Self-hosted** — Your data stays on your server, zero third-party dependency
- **i18n** — English and Chinese

## Quick Start

```bash
git clone https://github.com/gentpan/recalla.git
cd recalla
cp .env.example .env
docker compose up -d
```

Open `http://your-server:14200` — default login: `admin` / `admin123`

## Connect Your AI Tools

### Claude Code

```bash
claude mcp add-json recalla '{"type":"http","url":"https://your-server/mcp"}' --scope user
```

### Cursor

Settings → MCP → Add Server → URL: `https://your-server/mcp`

### Codex (OpenAI CLI)

```toml
# ~/.codex/config.toml
[mcp.recalla]
type = "url"
url  = "https://your-server/mcp"
```

### VS Code

```json
{
  "mcp": {
    "servers": {
      "recalla": {
        "url": "https://your-server/mcp",
        "type": "streamableHttp"
      }
    }
  }
}
```

## Auto-recall Rules

Add to your project's `CLAUDE.md` / `AGENTS.md` / `.cursorrules`:

```markdown
## Recalla Rules
- On conversation start: call context_restore
- On important decisions: call memory_save
- On conversation end: call session_sync
- Before new tasks: call memory_search
- After config changes: call config_push
```

## MCP Tools (12)

| Tool | Description |
|------|-------------|
| `memory_save` | Save a memory (auto-tagged, auto-scored) |
| `memory_search` | Semantic search across all memories |
| `context_restore` | Restore project context (last device, branch, recent work) |
| `session_sync` | Sync current AI session to cloud |
| `session_compress` | AI-compress long sessions (auto-saves as memory) |
| `project_list` | List all recorded projects |
| `config_push` | Push local AI config files to server |
| `config_pull` | Pull latest config from server |
| `team_search` | Search across all team members' memories |
| `team_share` | Share a memory with your team |
| `add_fact` | Add entity fact to knowledge graph |
| `query_facts` | Query knowledge graph facts |

## REST API

All endpoints require `Authorization: Bearer <api-key>` header. Generate API keys in Dashboard → Account.

### Memory

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/memory/save` | Save memory (auto-tagged) |
| POST | `/api/memory/search` | Semantic search |
| PUT | `/api/memory/{id}` | Update memory |
| DELETE | `/api/memory/{id}` | Delete memory |
| POST | `/api/memory/check` | Contradiction detection |

### Context & Sessions

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/context/restore` | Restore project context |
| POST | `/api/session/sync` | Sync session |
| POST | `/api/session/compress` | Compress session |
| GET | `/api/sessions` | List sessions |
| POST | `/api/sessions/import` | Bulk import sessions |

### Projects

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects` | List projects |
| GET | `/api/project/{name}` | Project detail |
| GET | `/api/project/{name}/memories` | Project memories |
| GET | `/api/project/{name}/sessions` | Project sessions |
| GET | `/api/stats` | Dashboard stats |

### Teams

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/teams` | Create team |
| GET | `/api/teams` | List my teams |
| GET | `/api/teams/{id}/detail` | Team detail (members, projects, activity) |
| POST | `/api/teams/{id}/invite` | Invite member by username |
| POST | `/api/teams/{id}/projects` | Link project to team |
| POST | `/api/teams/{id}/search` | Search team memories |
| POST | `/api/teams/{id}/share` | Share memory to team |
| GET | `/api/invites` | Pending invites |
| POST | `/api/invites/{id}/accept` | Accept invite |

### Knowledge Graph

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/kg/facts` | Add entity fact |
| GET | `/api/kg/facts` | Query facts (?subject=&predicate=&project=) |
| DELETE | `/api/kg/facts/{id}` | Invalidate fact |

### Config Sync

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/config/push` | Push config to server |
| POST | `/api/config/pull` | Pull config from server |
| GET | `/api/config/list` | List synced configs |

### Briefing

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/briefing` | Generate daily briefing |

### GitHub

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/github/repos` | List GitHub repos |
| GET | `/api/github/repos/{owner}/{repo}/commits` | Repo commits |
| POST | `/api/github/webhook` | GitHub webhook receiver |

### Auth

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/login` | Login |
| GET | `/api/auth/me` | Current user |
| POST | `/api/auth/password` | Change password |
| POST | `/api/auth/username` | Change username |
| GET | `/api/auth/keys` | List API keys |
| POST | `/api/auth/keys` | Generate API key |
| DELETE | `/api/auth/keys/{id}` | Delete API key |

## Architecture

```
┌─────────────────────────────────┐
│  Claude / Cursor / Codex / VSC  │
│         (Any Device)            │
└──────────┬──────────────────────┘
           │ MCP Protocol (12 tools)
           ↓
┌─────────────────────────────────┐
│       Recalla Server (Go)       │
│  REST API + MCP + Dashboard     │
│  + Telegram Bot + CLI           │
└──────┬─────────┬────────┬───────┘
       ↓         ↓        ↓
   Postgres   Qdrant   AI API
   (metadata) (vectors) (embed/compress)
```

## Tech Stack

- **Go** — API server (standard library, no framework)
- **PostgreSQL** — Structured storage (users, memories, sessions, teams, knowledge graph)
- **Qdrant** — Vector search for semantic memory retrieval
- **MCP** — Model Context Protocol (Anthropic standard)
- **Docker** — One-command deployment

## AI Provider Support

Configure in Dashboard → Settings. Multiple providers simultaneously.

| Provider | Embedding | LLM (Compression) | Note |
|----------|-----------|-------------------|------|
| OpenAI | text-embedding-3-small/large | gpt-4o-mini | Best quality |
| Qwen | text-embedding-v3 | qwen-plus | Good for Chinese, affordable |
| DeepSeek | Not supported | deepseek-chat | LLM only |
| Ollama | nomic-embed-text | Local models | Self-hosted |

## Telegram Bot

Commands: `/search` `/save` `/ask` `/context` `/projects` `/compress` `/briefing` `/help`

Setup: Create bot via @BotFather → Set `RECALLA_TELEGRAM_TOKEN` → Set webhook to `https://your-server/api/telegram/webhook`

## CLI Tool

```bash
go install github.com/gentpan/recalla/cmd/recalla@latest

export RECALLA_URL=https://your-server
export RECALLA_KEY=rk-xxx

recalla search "login bug"
recalla save "Chose PostgreSQL for JSON support"
recalla context zhanxing.io
recalla status
recalla push claude-md ~/.claude/CLAUDE.md
recalla pull claude-md
```

## Development

```bash
go run ./cmd/server/           # Run server
go run ./cmd/recalla/ help     # CLI help
go build ./cmd/server/         # Build server
go build ./cmd/recalla/        # Build CLI
GOOS=linux GOARCH=amd64 go build -o dist/recalla-server ./cmd/server/  # Cross-compile
```

## License

MIT
