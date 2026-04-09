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
- **MCP protocol** — One server works with Claude Code, Cursor, VS Code, Codex
- **GitHub integration** — Auto-record pushes and PRs as memories via Webhook
- **Multi-provider AI** — Support OpenAI, Qwen, DeepSeek, Ollama, or any OpenAI-compatible API
- **Dashboard** — Web UI to browse memories, sessions, projects, and manage settings
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
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `memory_save` | Save a memory (code, decision, note, conversation) |
| `memory_search` | Semantic search across all memories |
| `context_restore` | Restore project context (last device, branch, recent work) |
| `session_sync` | Sync current AI session to cloud |
| `session_compress` | AI-compress long sessions into structured summaries |
| `project_list` | List all recorded projects |

## REST API

All endpoints require `Authorization: Bearer <api-key>` header. Generate API keys in Dashboard → Account.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/memory/save` | Save memory |
| POST | `/api/memory/search` | Semantic search |
| DELETE | `/api/memory/{id}` | Delete memory |
| PUT | `/api/memory/{id}` | Update memory |
| POST | `/api/context/restore` | Restore project context |
| POST | `/api/session/sync` | Sync session |
| POST | `/api/session/compress` | Compress session |
| GET | `/api/projects` | List projects |
| GET | `/api/project/{name}` | Project detail |
| GET | `/api/stats` | Dashboard stats |

## Architecture

```
┌─────────────────────────────────┐
│  Claude / Cursor / Codex / VSC  │
│         (Any Device)            │
└──────────┬──────────────────────┘
           │ MCP Protocol
           ↓
┌─────────────────────────────────┐
│       Recalla Server (Go)       │
│  REST API + MCP + Dashboard     │
└──────┬─────────┬────────┬───────┘
       ↓         ↓        ↓
   Postgres   Qdrant   AI API
   (metadata) (vectors) (embed/compress)
```

## Tech Stack

- **Go** — API server (standard library, no framework)
- **PostgreSQL** — Structured storage (users, memories, sessions, projects)
- **Qdrant** — Vector search for semantic memory retrieval
- **MCP** — Model Context Protocol (Anthropic standard)
- **Docker** — One-command deployment

## AI Provider Support

Configure in Dashboard → Settings. Multiple providers can be added simultaneously.

| Provider | Embedding | LLM (Compression) | Note |
|----------|-----------|-------------------|------|
| OpenAI | text-embedding-3-small/large | gpt-4o-mini | Best quality |
| Qwen | text-embedding-v3 | qwen-plus | Good for Chinese, affordable |
| DeepSeek | Not supported | deepseek-chat | LLM only |
| Ollama | nomic-embed-text | Local models | Self-hosted |

## GitHub Integration

1. Add your GitHub Token in Settings
2. Browse repos and commits in Dashboard → GitHub
3. Set up Webhook (`https://your-server/api/github/webhook`) to auto-record:
   - Push events → saved as `code` type memories
   - PR merged/opened → saved as `code` type memories

## Development

```bash
# Local dev
go run ./cmd/server/

# Build
go build -o dist/recalla-server ./cmd/server/

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o dist/recalla-server ./cmd/server/
```

## License

MIT
